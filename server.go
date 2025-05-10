package main

import (
	"encoding/json"
	//"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings" // Añadir si usas strings.Contains en el manejo de errores de ReadMessage
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	id             string
	conn           *websocket.Conn
	peerConnection *webrtc.PeerConnection
}

type Server struct {
	clients       map[string]*Client
	clientsMutex  sync.Mutex
	mediaManager  *MediaManager
	webRTCManager *WebRTCManager
}

func NewServer(mm *MediaManager, wm *WebRTCManager) *Server {
	if mm == nil {
		log.Fatal("Server: MediaManager no puede ser nil")
	}
	if wm == nil {
		log.Fatal("Server: WebRTCManager no puede ser nil")
	}
	return &Server{
		clients:       make(map[string]*Client),
		mediaManager:  mm,
		webRTCManager: wm,
	}
}

func (s *Server) RegisterHandlers() {
	http.HandleFunc("/", s.serveClientHTML)
	http.HandleFunc("/ws", s.handleWebSocket)
}

func (s *Server) Start(addr string) error {
	log.Printf("Servidor HTTP/WebSocket iniciando en %s", addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) serveClientHTML(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(htmlFilePath); os.IsNotExist(err) { // htmlFilePath de config.go
		http.Error(w, "client.html no encontrado", http.StatusNotFound)
		log.Printf("Error sirviendo HTML: %s no encontrado\n", htmlFilePath)
		return
	}
	http.ServeFile(w, r, htmlFilePath)
}

func (s *Server) addClient(client *Client) {
	s.clientsMutex.Lock()
	s.clients[client.id] = client
	log.Printf("[%s] Cliente añadido. Total: %d", client.id, len(s.clients))
	s.clientsMutex.Unlock()
}

func (s *Server) removeClient(clientID string) {
	s.clientsMutex.Lock()
	client, exists := s.clients[clientID]
	delete(s.clients, clientID) // Siempre eliminar del mapa
	s.clientsMutex.Unlock()     // Desbloquear antes de operaciones potencialmente largas

	if exists {
		log.Printf("[%s] Cliente eliminado. Total restantes: %d", clientID, len(s.clients)-1) // -1 es un error, len(s.clients) ya estará actualizado
		if client.peerConnection != nil && client.peerConnection.ConnectionState() != webrtc.PeerConnectionStateClosed {
			log.Printf("Server: Cerrando PeerConnection para %s en removeClient", clientID)
			if err := client.peerConnection.Close(); err != nil {
				log.Printf("Server: Error al cerrar PeerConnection de %s: %v", clientID, err)
			}
		}
		// La conexión WebSocket conn.Close() generalmente se maneja en el defer de handleWebSocket
		// o cuando el bucle de lectura ReadMessage falla.
	} else {
		log.Printf("Server: Intento de eliminar cliente %s que no existía.", clientID)
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil { log.Printf("Fallo al actualizar a WebSocket: %v", err); return }

	clientID := uuid.NewString()
	log.Printf("[%s] Cliente WebSocket conectado.", clientID)

	peerConnection, err := s.webRTCManager.NewPeerConnection()
	if err != nil {
		log.Printf("[%s] Fallo al crear PeerConnection: %v", clientID, err)
		conn.Close(); return
	}

	client := &Client{id: clientID, conn: conn, peerConnection: peerConnection}
	s.addClient(client)

	defer func() {
		log.Printf("[%s] Iniciando limpieza (defer).", clientID)
		// El PeerConnection se cierra aquí si no se ha cerrado antes
		// por un cambio de estado a Failed/Closed/Disconnected.
		if peerConnection.ConnectionState() != webrtc.PeerConnectionStateClosed {
			log.Printf("[%s] Cerrando PeerConnection en defer.", clientID)
			if err := peerConnection.Close(); err != nil {
				log.Printf("[%s] Error cerrando PeerConnection en defer: %v", clientID, err)
			}
		}
		// conn.Close() // El websocket se cierra si ReadMessage falla o si OnConnectionStateChange lo cierra.
		s.removeClient(clientID)
		log.Printf("[%s] Limpieza completada.", clientID)
	}()

	var tracksAdded []string
	if videoTrack, ok := s.mediaManager.GetVideoTrack(); ok {
		if _, err = peerConnection.AddTrack(videoTrack); err == nil {
			tracksAdded = append(tracksAdded, "Video")
		} else { log.Printf("[%s] Fallo al añadir pista de video: %v", clientID, err) }
	}
	if audioTrack, ok := s.mediaManager.GetAudioTrack(); ok {
		if _, err = peerConnection.AddTrack(audioTrack); err == nil {
			tracksAdded = append(tracksAdded, "Audio")
		} else { log.Printf("[%s] Fallo al añadir pista de audio: %v", clientID, err) }
	}

	if len(tracksAdded) > 0 {
		log.Printf("[%s] Pistas compartidas añadidas al PeerConnection: %v", clientID, tracksAdded)
	} else {
		log.Printf("[%s] ADVERTENCIA: No se añadieron pistas al PeerConnection. Verifique captura.", clientID)
		// No retornamos aquí, ya que el cliente podría querer conectarse incluso sin media (aunque no es el caso de uso actual)
	}

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil { log.Printf("[%s] ICE finalizado.", clientID); return }
		log.Printf("[%s] Nuevo ICE local.", clientID)
		payload, errMarshal := json.Marshal(map[string]interface{}{"type": "candidate", "candidate": candidate.ToJSON()})
		if errMarshal != nil { log.Printf("[%s] Error serializando candidato ICE: %v", clientID, errMarshal); return }
		
		// Escribir en la conexión actual. Si falla, el bucle de lectura lo detectará.
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("[%s] Error enviando candidato ICE: %v", clientID, err)
		}
	})

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("[%s] PeerConnection state: %s", clientID, state.String())
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateDisconnected {
			log.Printf("[%s] PeerConnection cerrado/fallido/desconectado. Cerrando WebSocket.", clientID)
			conn.Close() // Esto terminará el bucle ReadMessage
		}
	})

	for {
		_, message, errRead := conn.ReadMessage()
		if errRead != nil {
			if websocket.IsCloseError(errRead, websocket.CloseNormalClosure, websocket.CloseGoingAway) || errRead == io.ErrUnexpectedEOF {
				log.Printf("[%s] Cliente WebSocket desconectado (esperado).", clientID)
			} else if opErr, ok := errRead.(*net.OpError); ok && (opErr.Err.Error() == "use of closed network connection" || strings.Contains(opErr.Err.Error(), "connection reset by peer")) {
				log.Printf("[%s] Cliente WebSocket desconectado (red).", clientID)
			} else if websocket.IsUnexpectedCloseError(errRead, websocket.CloseNormalClosure, websocket.CloseGoingAway) { // Excluir cierres normales
				log.Printf("[%s] Error WebSocket inesperado: %v", clientID, errRead)
			} else {
				log.Printf("[%s] Error leyendo mensaje WebSocket: %v", clientID, errRead)
			}
			break
		}

		var msg map[string]interface{}
		if errJson := json.Unmarshal(message, &msg); errJson != nil {
			log.Printf("[%s] Error deserializando JSON: %v (mensaje: %s)", clientID, errJson, string(message)); continue
		}
		log.Printf("[%s] Mensaje WS: Tipo '%s'", clientID, msg["type"])

		switch msg["type"] {
		case "offer":
			sdpData, okSdpData := msg["sdp"].(map[string]interface{})
			if !okSdpData { log.Printf("[%s] Error: 'sdp' no es objeto.", clientID); continue }
			sdpString, okSdpStr := sdpData["sdp"].(string)
			if !okSdpStr { log.Printf("[%s] Error: 'sdp.sdp' no es string.", clientID); continue }

			offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdpString}
			if err = peerConnection.SetRemoteDescription(offer); err != nil {
				log.Printf("[%s] Fallo SetRemoteDesc(offer): %v", clientID, err); continue
			}
			log.Printf("[%s] RemoteDesc(offer) establecido.", clientID)

			answer, errAns := peerConnection.CreateAnswer(nil)
			if errAns != nil { log.Printf("[%s] Fallo CreateAnswer: %v", clientID, errAns); continue }

			gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
			if err = peerConnection.SetLocalDescription(answer); err != nil {
				log.Printf("[%s] Fallo SetLocalDesc(answer): %v", clientID, err); continue
			}
			log.Printf("[%s] LocalDesc(answer) establecido.", clientID)

			go func(pcToUse *webrtc.PeerConnection, connToUse *websocket.Conn, currentClientID string) {
				select {
				case <-time.After(5 * time.Second): log.Printf("[%s] Timeout ICE para respuesta.", currentClientID)
				case <-gatherComplete: log.Printf("[%s] Recolección ICE completa para respuesta.", currentClientID)
				}
				localDesc := pcToUse.LocalDescription()
				if localDesc == nil { log.Printf("[%s] LocalDesc nulo post-ICE.", currentClientID); return }
				payload, errMrsh := json.Marshal(map[string]interface{}{"type": "answer", "sdp": localDesc})
				if errMrsh != nil { log.Printf("[%s] Fallo Marshal Answer: %v", currentClientID, errMrsh); return }

				// Escribir en la conexión específica. La conexión 'connToUse' es la que se pasó a la goroutine.
				if errWr := connToUse.WriteMessage(websocket.TextMessage, payload); errWr != nil {
					log.Printf("[%s] Fallo envío Answer SDP: %v", currentClientID, errWr)
				} else {
					log.Printf("[%s] Respuesta SDP enviada.", currentClientID)
				}
			}(peerConnection, conn, clientID)

		case "candidate":
			candidateData, okCandData := msg["candidate"].(map[string]interface{})
			if !okCandData || candidateData["candidate"] == nil { log.Printf("[%s] 'candidate' malformado.", clientID); continue }
			candidateStr, okCandStr := candidateData["candidate"].(string)
			if !okCandStr { log.Printf("[%s] 'candidate.candidate' no string.", clientID); continue }
			candidate := webrtc.ICECandidateInit{Candidate: candidateStr}
			if sdpMLineIndex, ok := candidateData["sdpMLineIndex"].(float64); ok { idx := uint16(sdpMLineIndex); candidate.SDPMLineIndex = &idx }
			if sdpMid, ok := candidateData["sdpMid"].(string); ok { candidate.SDPMid = &sdpMid }
			if err = peerConnection.AddICECandidate(candidate); err != nil {
				log.Printf("[%s] Fallo AddICECandidate: %v", clientID, err)
			} else {
				log.Printf("[%s] Candidato ICE remoto añadido.", clientID)
			}
		default:
			log.Printf("[%s] Tipo de mensaje desconocido: %s", clientID, msg["type"])
		}
	}
	log.Printf("[%s] Saliendo del bucle de mensajes WebSocket.", clientID)
}
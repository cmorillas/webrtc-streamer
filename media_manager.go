package main

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/opus"
	"github.com/pion/mediadevices/pkg/codec/vpx"
	"github.com/pion/mediadevices/pkg/prop"
	// Los drivers se importan en main.go para EnumerateDevices,
	// pero es bueno tenerlos aquí también si este paquete se usara de forma más aislada.
	// _ "github.com/pion/mediadevices/pkg/driver/camera"
	// _ "github.com/pion/mediadevices/pkg/driver/microphone"
)

type MediaManager struct {
	mutex            sync.RWMutex
	mediaStream      mediadevices.MediaStream
	videoTrack       mediadevices.Track
	audioTrack       mediadevices.Track
	isVideoEnabled   bool
	isAudioEnabled   bool
	codecSelector    *mediadevices.CodecSelector
}

func NewMediaManager() *MediaManager {
	return &MediaManager{}
}

func (m *MediaManager) Initialize(cfg *Config) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	log.Println("MediaManager: Inicializando...")

	if cfg.VideoDeviceID == "" && cfg.AudioDeviceID == "" {
		return errors.New("MediaManager: no se especificaron dispositivos válidos para capturar")
	}

	var codecSelectorOptions []mediadevices.CodecSelectorOption
	if cfg.VideoDeviceID != "" {
		vp8Params, errVP8 := vpx.NewVP8Params()
		if errVP8 != nil { return fmt.Errorf("MediaManager: fallo al crear params VP8: %w", errVP8) }
		vp8Params.BitRate = 1_500_000
		//vp8Params.BitRate = 700_000
		codecSelectorOptions = append(codecSelectorOptions, mediadevices.WithVideoEncoders(&vp8Params))
		m.isVideoEnabled = true
		log.Println("MediaManager: Codec VP8 para video habilitado.")
	}
	if cfg.AudioDeviceID != "" {
		opusParams, errOpus := opus.NewParams()
		if errOpus != nil { return fmt.Errorf("MediaManager: fallo al crear params Opus: %w", errOpus) }
		codecSelectorOptions = append(codecSelectorOptions, mediadevices.WithAudioEncoders(&opusParams))
		m.isAudioEnabled = true
		log.Println("MediaManager: Codec Opus para audio habilitado.")
	}

	if len(codecSelectorOptions) == 0 { return errors.New("MediaManager: no se configuraron codecs") }
	m.codecSelector = mediadevices.NewCodecSelector(codecSelectorOptions...)

	constraints := mediadevices.MediaStreamConstraints{ Codec: m.codecSelector } // Desreferenciar
	logStreamMsg := "MediaManager: Intentando obtener MediaStream ("
	hasRequest := false

	if m.isVideoEnabled {
		constraints.Video = func(c *mediadevices.MediaTrackConstraints) {
			c.DeviceID = prop.String(cfg.VideoDeviceID)
			//c.Width = prop.Int(1024)
			//c.Height = prop.Int(818)
			//c.Width = prop.Int(800)
			//c.Height = prop.Int(600)
			log.Printf("MediaManager: Constraint Video: DeviceID='%s'", cfg.VideoDeviceID)
		}
		logStreamMsg += fmt.Sprintf("Video desde '%s'", cfg.VideoDeviceID); hasRequest = true
	}
	if m.isAudioEnabled {
		constraints.Audio = func(c *mediadevices.MediaTrackConstraints) {
			c.DeviceID = prop.String(cfg.AudioDeviceID)
			log.Printf("MediaManager: Constraint Audio: DeviceID='%s'", cfg.AudioDeviceID)
		}
		if hasRequest { logStreamMsg += " y " }
		logStreamMsg += fmt.Sprintf("Audio desde '%s'", cfg.AudioDeviceID); hasRequest = true
	}
	logStreamMsg += ")..."

	var lastErr error
	for i := 0; i < mediaCaptureRetries; i++ {
		log.Printf("%s (Intento %d/%d)", logStreamMsg, i+1, mediaCaptureRetries)
		m.mediaStream, lastErr = mediadevices.GetUserMedia(constraints)
		if lastErr == nil && m.mediaStream != nil {
			log.Println("MediaManager: MediaStream obtenido exitosamente.")
			break
		}
        if m.mediaStream != nil { // Si hubo stream pero también error, limpiar
            for _, track := range m.mediaStream.GetTracks() { track.Close() }
            m.mediaStream = nil
        }
		log.Printf("MediaManager: Fallo al obtener MediaStream (intento %d): %v", i+1, lastErr)
		if i < mediaCaptureRetries-1 {
			retryDelay := time.Duration(mediaCaptureRetryDelaySeconds) * time.Second
			log.Printf("MediaManager: Esperando %v antes de reintentar...", retryDelay)
			time.Sleep(retryDelay)
		}
	}

	if lastErr != nil || m.mediaStream == nil {
		return fmt.Errorf("MediaManager: fallo al obtener MediaStream después de %d intentos: %w", mediaCaptureRetries, lastErr)
	}

	// Extraer pistas
	if m.isVideoEnabled {
		videoTracks := m.mediaStream.GetVideoTracks()
		if len(videoTracks) > 0 {
			m.videoTrack = videoTracks[0]
			log.Printf("MediaManager: Pista de video compartida inicializada: ID=%s", m.videoTrack.ID())
		} else {
			log.Println("MediaManager ADVERTENCIA: Se solicitó video pero no se obtuvo pista de video.")
			m.isVideoEnabled = false // Corregir el flag si no se obtuvo
		}
	}
	if m.isAudioEnabled {
		audioTracks := m.mediaStream.GetAudioTracks()
		if len(audioTracks) > 0 {
			m.audioTrack = audioTracks[0]
			log.Printf("MediaManager: Pista de audio compartida inicializada: ID=%s", m.audioTrack.ID())
		} else {
			log.Println("MediaManager ADVERTENCIA: Se solicitó audio pero no se obtuvo pista de audio.")
			m.isAudioEnabled = false // Corregir el flag si no se obtuvo
		}
	}
    
	if !m.isVideoEnabled && !m.isAudioEnabled {
		if m.mediaStream != nil { for _, track := range m.mediaStream.GetTracks() { track.Close() } }
		return errors.New("MediaManager: no se pudo obtener ninguna pista de medios solicitada después de los reintentos")
	}
	log.Println("MediaManager inicializado exitosamente.")
	return nil
}

func (m *MediaManager) GetVideoTrack() (mediadevices.Track, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.videoTrack, m.isVideoEnabled && m.videoTrack != nil
}

func (m *MediaManager) GetAudioTrack() (mediadevices.Track, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.audioTrack, m.isAudioEnabled && m.audioTrack != nil
}

func (m *MediaManager) GetCodecSelector() *mediadevices.CodecSelector {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    return m.codecSelector
}

func (m *MediaManager) Close() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.mediaStream != nil {
		log.Println("MediaManager: Cerrando MediaStream compartido...")
		for _, track := range m.mediaStream.GetTracks() {
			if err := track.Close(); err != nil {
				log.Printf("MediaManager: Error cerrando track %s: %v", track.ID(), err)
			}
		}
		m.mediaStream = nil
		m.videoTrack = nil
		m.audioTrack = nil
		log.Println("MediaManager: MediaStream compartido cerrado.")
	}
}
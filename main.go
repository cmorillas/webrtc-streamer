package main

import (
	"encoding/hex"
	// "flag" // No se usa directamente aquí, loadConfig lo maneja
	"fmt"
	"log"
	"os"

	"github.com/pion/mediadevices"
	// Drivers necesarios para que EnumerateDevices funcione correctamente al inicio
	_ "github.com/pion/mediadevices/pkg/driver/camera"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
)

const (
	port = "8080" // Puerto del servidor
)

// Funciones helper que se usan en la lógica de inicialización de main
func mediaDeviceTypeToString(kind mediadevices.MediaDeviceType) string {
	switch kind {
	case mediadevices.AudioInput:
		return "AudioInput"
	case mediadevices.VideoInput:
		return "VideoInput"
	default:
		return "Unknown"
	}
}

func findDevice(identifier string, deviceKind mediadevices.MediaDeviceType, allDevices []mediadevices.MediaDeviceInfo) (string, bool) {
	if identifier == "" {
		return "", false
	}
	// Coincidencia por ID
	for _, dev := range allDevices {
		if dev.DeviceID == identifier && dev.Kind == deviceKind {
			log.Printf("Dispositivo encontrado por ID: '%s' (%s), Label: '%s'", dev.DeviceID, mediaDeviceTypeToString(dev.Kind), dev.Label)
			return dev.DeviceID, true
		}
	}
	// Coincidencia por Label
	for _, dev := range allDevices {
		if dev.Label == identifier && dev.Kind == deviceKind {
			log.Printf("Dispositivo encontrado por Label: '%s' -> ID real: '%s' (%s)", identifier, dev.DeviceID, mediaDeviceTypeToString(dev.Kind))
			return dev.DeviceID, true
		}
	}
	// Coincidencia por Label decodificado (hex)
	for _, dev := range allDevices {
		if dev.Kind == deviceKind {
			decodedLabelBytes, err := hex.DecodeString(dev.Label) // hex.DecodeString es de encoding/hex
			if err == nil {
				if string(decodedLabelBytes) == identifier {
					log.Printf("Dispositivo encontrado por Label decodificado (hex): '%s' -> ID real: '%s' (%s)", identifier, dev.DeviceID, mediaDeviceTypeToString(dev.Kind))
					return dev.DeviceID, true
				}
			}
		}
	}
	return "", false
}

func main() {
	cfg := loadConfig() // Carga desde config.go

	if cfg.ListDevices {
		fmt.Println("Detectando dispositivos multimedia con mediadevices...")
		allAvailableDevices := mediadevices.EnumerateDevices()
		fmt.Println("Dispositivos encontrados:")
		if len(allAvailableDevices) == 0 {
			fmt.Println("  No se encontraron dispositivos.")
			os.Exit(0)
		}
		for _, dev := range allAvailableDevices {
			displayLabel := dev.Label
			decodedBytes, err := hex.DecodeString(dev.Label)
			if err == nil && len(decodedBytes) > 0 {
				isPrintable := true
				for _, b := range decodedBytes {
					if b < 32 || b > 126 { // Caracteres ASCII imprimibles básicos
						isPrintable = false
						break
					}
				}
				if isPrintable {
					displayLabel = fmt.Sprintf("%s (hex: %s)", string(decodedBytes), dev.Label)
				}
			}
			fmt.Printf("  - Tipo: %s, ID: '%s', Label: '%s'\n", mediaDeviceTypeToString(dev.Kind), dev.DeviceID, displayLabel)
		}
		os.Exit(0)
	}

	// Lógica Normal del Programa (si --list-devices no está presente)
	if cfg.VideoIdentifier == "" && cfg.AudioIdentifier == "" {
		log.Fatal("Error: Debes especificar -v <id_o_label> y/o -a <id_o_label>, o usar --list-devices.")
	}
	log.Printf("Solicitado: Video='%s', Audio='%s'\n", cfg.VideoIdentifier, cfg.AudioIdentifier)

	// Enumerar (de nuevo, necesario para la lógica normal si no se hizo antes para listar)
	allAvailableDevices := mediadevices.EnumerateDevices()

	// Buscar y validar dispositivos
	if cfg.VideoIdentifier != "" {
		var found bool
		cfg.VideoDeviceID, found = findDevice(cfg.VideoIdentifier, mediadevices.VideoInput, allAvailableDevices)
		if !found {
			log.Fatalf("Error: Dispositivo de video '%s' no encontrado.", cfg.VideoIdentifier)
		}
	}
	if cfg.AudioIdentifier != "" {
		var found bool
		cfg.AudioDeviceID, found = findDevice(cfg.AudioIdentifier, mediadevices.AudioInput, allAvailableDevices)
		if !found {
			log.Fatalf("Error: Dispositivo de audio '%s' no encontrado.", cfg.AudioIdentifier)
		}
	}
	log.Printf("IDs reales a usar: Video='%s', Audio='%s'\n", cfg.VideoDeviceID, cfg.AudioDeviceID)

	// Iniciar MediaManager
	mediaManager := NewMediaManager() // Definido en media_manager.go
	if err := mediaManager.Initialize(cfg); err != nil {
		log.Fatalf("Error crítico al iniciar MediaManager: %v", err)
	}
	defer mediaManager.Close() // Asegurar que los medios se cierren al final

	// Iniciar WebRTCManager
	codecSelectorForWebRTC := mediaManager.GetCodecSelector()
	if codecSelectorForWebRTC == nil {
		log.Fatal("MediaManager no proporcionó un CodecSelector válido.")
	}
	webRTCManager, err := NewWebRTCManager(codecSelectorForWebRTC) // Definido en webrtc_manager.go
	if err != nil {
		log.Fatalf("Error crítico al iniciar WebRTCManager: %v", err)
	}

	// Crear e iniciar el servidor
	srv := NewServer(mediaManager, webRTCManager) // Definido en server.go
	srv.RegisterHandlers()

	log.Printf("Servidor HTTP/WebSocket iniciado en http://localhost:%s", port)
	if err := srv.Start(":" + port); err != nil {
		log.Fatalf("Fallo al iniciar servidor HTTP: %v", err)
	}

	log.Println("Servidor detenido.")
}
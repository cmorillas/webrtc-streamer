package main

import "flag"

// Constantes que podrían ser configurables o usadas en múltiples lugares.
const (
	htmlFilePath                = "./client.html" // Ruta al archivo HTML del cliente
	mediaCaptureRetries         = 5               // Número de reintentos para GetUserMedia
	mediaCaptureRetryDelaySeconds = 5               // Retraso en segundos entre reintentos de captura
)

// Config almacena la configuración obtenida de los flags de línea de comandos.
type Config struct {
	ListDevices     bool   // Si es true, lista dispositivos y sale
	VideoIdentifier string // Identificador (ID o Label) para el video del flag -v
	AudioIdentifier string // Identificador (ID o Label) para el audio del flag -a
	VideoDeviceID   string // El DeviceID real resuelto para el video
	AudioDeviceID   string // El DeviceID real resuelto para el audio
}

// loadConfig parsea los flags de línea de comandos y devuelve un struct Config.
func loadConfig() *Config {
	listDevicesFlag := flag.Bool("list-devices", false, "Lista dispositivos multimedia detectados por mediadevices y sale.")
	videoDeviceArg := flag.String("v", "", "ID o Label del dispositivo de video a usar.")
	audioDeviceArg := flag.String("a", "", "ID o Label del dispositivo de audio a usar.")
	flag.Parse()

	return &Config{
		ListDevices:     *listDevicesFlag,
		VideoIdentifier: *videoDeviceArg,
		AudioIdentifier: *audioDeviceArg,
		// VideoDeviceID y AudioDeviceID se llenarán en main.go después de la validación
	}
}
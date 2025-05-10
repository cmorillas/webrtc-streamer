package main

import (
	"errors"
	"log"

	"github.com/pion/mediadevices" // Necesario para el tipo CodecSelector
	"github.com/pion/webrtc/v4"
)

type WebRTCManager struct {
	api *webrtc.API
}

func NewWebRTCManager(codecSelector *mediadevices.CodecSelector) (*WebRTCManager, error) {
	if codecSelector == nil {
		return nil, errors.New("WebRTCManager: CodecSelector no puede ser nil para inicializar")
	}
	mediaEngine := &webrtc.MediaEngine{}
	codecSelector.Populate(mediaEngine) // Populate usa el valor, aunque codecSelector sea un puntero
	log.Println("WebRTCManager: MediaEngine populado con codecs.")
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	return &WebRTCManager{api: api}, nil
}

func (m *WebRTCManager) NewPeerConnection() (*webrtc.PeerConnection, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
	return m.api.NewPeerConnection(config)
}

---

**Especificaciones Técnicas: Servidor Go WebRTC para Streaming Multimedia**

**Versión 1.0** 

**1. Propósito General**

Este programa implementa un servidor en Go que captura audio y/o video desde dispositivos específicos del sistema (cámara, micrófono, fuentes virtuales como OBS) y los transmite en tiempo real a múltiples clientes web utilizando WebRTC. Utiliza WebSockets para la señalización necesaria para establecer las conexiones WebRTC.

**2. Características Principales**

*   **Streaming Multimedia:** Soporta la transmisión de video, audio, o ambos simultáneamente.
*   **Selección de Dispositivos:** Permite al usuario especificar qué dispositivo de video y/o audio utilizar mediante flags en la línea de comandos.
*   **Identificación Flexible de Dispositivos:** Acepta tanto el `DeviceID` interno asignado por la librería `mediadevices` como el `Label` (nombre descriptivo) del dispositivo como identificadores en los flags, priorizando la coincidencia por ID, y también por `Label` decodificado de hexadecimal si aplica.
*   **Descubrimiento de Dispositivos:** Incluye un modo (`--list-devices`) para enumerar los dispositivos multimedia detectados por `mediadevices`, mostrando su `Tipo`, `DeviceID` y `Label` (con intento de decodificación hexadecimal para mejor legibilidad), facilitando la configuración de los flags.
*   **Soporte Multicliente:** Diseñado para manejar múltiples clientes web conectados simultáneamente, transmitiendo la misma fuente de medios compartida a todos ellos.
*   **Codecs Estándar:**
    *   **Video:** Utiliza el codec VP8 (configurable bitrate, por defecto 1.5 Mbps).
    *   **Audio:** Utiliza el codec Opus.
*   **Señalización WebRTC:** Implementa la negociación básica Offer/Answer y el intercambio de candidatos ICE a través de una conexión WebSocket por cliente.
*   **Servidor Web Integrado:** Sirve un archivo HTML/JavaScript (`client.html`) que actúa como cliente WebRTC receptor.
*   **Baja Latencia:** Aprovecha WebRTC para una transmisión con latencia relativamente baja.
*   **Robustez Inicial de Captura:** Intenta la captura de medios múltiples veces con un retraso si falla inicialmente, antes de terminar.
*   **Basado en Pion:** Utiliza las librerías `pion/webrtc` y `pion/mediadevices`.
*   **Estructura Modular:** El código del servidor está organizado en componentes lógicos (configuración, gestión de medios, gestión de WebRTC, servidor principal) para mejorar la mantenibilidad.

**3. Pila Tecnológica**

*   **Lenguaje:** Go (Golang)
*   **Librerías WebRTC/Media (Go):**
    *   `github.com/pion/webrtc/v4` (Core WebRTC)
    *   `github.com/pion/mediadevices` (Abstracción de dispositivos y codecs)
    *   `github.com/pion/mediadevices/pkg/codec/vpx` (Encoder VP8)
    *   `github.com/pion/mediadevices/pkg/codec/opus` (Encoder Opus)
    *   `github.com/pion/mediadevices/pkg/driver/camera` (Drivers de cámara)
    *   `github.com/pion/mediadevices/pkg/driver/microphone` (Drivers de micrófono)
    *   `github.com/pion/mediadevices/pkg/prop` (Propiedades de constraints)
*   **Librería WebSocket (Go):** `github.com/gorilla/websocket`
*   **Identificadores Únicos:** `github.com/google/uuid`
*   **Servidor HTTP (Go):** `net/http` (Librería estándar)
*   **Cliente:** HTML5, JavaScript (Web API: `RTCPeerConnection`, `WebSocket`, `MediaStream`)
*   **Señalización:** Protocolo simple basado en JSON sobre WebSocket (mensajes `offer`, `answer`, `candidate`).
*   **NAT Traversal:** Utiliza un servidor STUN público (`stun:stun.l.google.com:19302`).

**4. Arquitectura del Servidor (Go)**

El servidor está estructurado en varios componentes lógicos, cada uno típicamente encapsulado en su propio archivo para mejorar la organización y mantenibilidad:

*   **`config.go` (`Config` struct y `loadConfig()`):**
    *   Define la estructura `Config` para almacenar los parámetros de configuración obtenidos de la línea de comandos.
    *   Contiene la función `loadConfig()` que utiliza el paquete `flag` de Go para parsear los argumentos de línea de comandos (`-v`, `-a`, `--list-devices`) y poblar el struct `Config`.
*   **`main.go` (`main()` function y helpers de inicialización):**
    *   Punto de entrada de la aplicación.
    *   Llama a `loadConfig()` para obtener la configuración.
    *   Si se especifica el flag `--list-devices`, utiliza `mediadevices.EnumerateDevices()` y helpers locales (`mediaDeviceTypeToString`, `findDevice`) para listar los dispositivos multimedia disponibles y sus `DeviceID` y `Label`, luego termina.
    *   Para la ejecución normal, valida los identificadores de dispositivo proporcionados en `Config` contra la lista de dispositivos enumerados (usando `findDevice`). Termina si un dispositivo especificado no se encuentra. Actualiza `cfg.VideoDeviceID` y `cfg.AudioDeviceID` con los IDs reales.
    *   Crea e inicializa una instancia de `MediaManager`, pasándole la configuración validada.
    *   Obtiene el `CodecSelector` del `MediaManager` y lo usa para crear e inicializar una instancia de `WebRTCManager`.
    *   Crea una instancia del `Server`, inyectando las instancias de `MediaManager` y `WebRTCManager`.
    *   Registra los manejadores HTTP/WebSocket del `Server`.
    *   Inicia el servidor HTTP.
    *   Maneja el cierre del `MediaManager` cuando el programa principal termina (vía `defer`).
*   **`media_manager.go` (`MediaManager` struct y métodos):**
    *   Responsable de la captura y gestión del stream de medios compartido.
    *   El método `Initialize()`:
        *   Configura el `mediadevices.CodecSelector` con los codecs VP8 y/o Opus según la configuración.
        *   Llama a `mediadevices.GetUserMedia()` con las constraints apropiadas (incluyendo los `DeviceID` validados de `Config`) para obtener el `MediaStream` compartido. Implementa una lógica de reintentos si la obtención inicial falla.
        *   Extrae y almacena internamente las pistas de video y audio (`mediadevices.Track`) del stream compartido.
    *   Proporciona métodos para acceder de forma segura (protegida por mutex) a las pistas compartidas (`GetVideoTrack()`, `GetAudioTrack()`) y al `CodecSelector`.
    *   El método `Close()` se encarga de cerrar el `MediaStream` y sus pistas.
*   **`webrtc_manager.go` (`WebRTCManager` struct y métodos):**
    *   Responsable de la configuración y creación de instancias relacionadas con WebRTC.
    *   El constructor `NewWebRTCManager()` recibe un `CodecSelector` (del `MediaManager`).
    *   Configura un `webrtc.MediaEngine` utilizando el `CodecSelector` proporcionado, registrando los codecs.
    *   Crea una instancia de `webrtc.API` con el `MediaEngine` configurado.
    *   Proporciona un método `NewPeerConnection()` que usa el `webrtc.API` para crear nuevas instancias de `webrtc.PeerConnection`.
*   **`server.go` (`Server` struct, `Client` struct, y métodos):**
    *   Define el struct `Server` que mantiene el estado de los clientes conectados y las referencias a `MediaManager` y `WebRTCManager`.
    *   Define el struct `Client` para almacenar la conexión WebSocket y el `PeerConnection` de cada cliente.
    *   El método `RegisterHandlers()` configura los endpoints HTTP (`/` para `client.html` y `/ws` para WebSockets).
    *   El método `Start()` inicia el servidor HTTP.
    *   El método `serveClientHTML()` sirve el archivo estático del cliente.
    *   El método `handleWebSocket()`:
        *   Se ejecuta en una nueva goroutine por cada cliente que se conecta vía WebSocket.
        *   Utiliza `WebRTCManager` para crear una instancia *nueva* de `webrtc.PeerConnection` para este cliente.
        *   Genera un ID único para el cliente, crea una instancia de `Client`, y la añade a un mapa interno (`clients`) protegido por mutex.
        *   Utiliza `MediaManager` para obtener las *pistas compartidas* y las añade al `PeerConnection` del cliente.
        *   Configura los callbacks (`OnICECandidate`, `OnConnectionStateChange`) para este `PeerConnection`.
        *   Entra en un bucle para leer mensajes JSON (offer, candidate) del cliente WebSocket.
        *   Procesa la oferta, crea una respuesta (answer), y la envía de vuelta al cliente (gestionando la espera de ICE de forma no bloqueante en una goroutine separada para el envío de la respuesta).
        *   Añade los candidatos ICE remotos recibidos del cliente.
        *   Utiliza un `defer` para cerrar el `PeerConnection` del cliente y llamar a `removeClient()` cuando la conexión WebSocket se cierra o falla.
    *   Los métodos `addClient()` y `removeClient()` gestionan el mapa de clientes de forma segura.

**5. Cliente (`client.html`)**

*   Una página HTML simple con un elemento `<video autoplay playsinline controls muted>`.
*   Código JavaScript que:
    *   Establece una conexión WebSocket con el servidor (`/ws`).
    *   Al conectarse, crea un `RTCPeerConnection`.
    *   Configura transceptores para *recibir* (`recvonly`) audio y video.
    *   Crea una oferta SDP (`createOffer`) y la envía al servidor vía WebSocket.
    *   Maneja los mensajes del servidor:
        *   Al recibir la `answer`, la establece como descripción remota (`setRemoteDescription`).
        *   Al recibir un `candidate`, lo añade al `PeerConnection` (`addIceCandidate`).
    *   Configura `pc.ontrack` para manejar las pistas de audio/video entrantes:
        *   Crea un único `MediaStream` la primera vez que se establece la conexión (o lo reinicia).
        *   Añade los `event.track` recibidos a ese `MediaStream` persistente.
        *   Asigna este `MediaStream` al `srcObject` del elemento `<video>` una sola vez (o si cambia el stream).
        *   Llama a `video.play()` para iniciar la reproducción.

**6. Uso del Programa**

*   **Compilación:**
    ```bash
    # (Opcional, si es la primera vez o cambian dependencias)
    # go mod init <nombre_del_modulo> 
    go mod tidy
    # Compilar
    go build -o webrtc-streamer . 
    ```
    (Reemplaza `webrtc-streamer` con el nombre de ejecutable deseado).

*   **Descubrir Dispositivos:**
    Antes de ejecutar el servidor, para conocer los identificadores de dispositivo que `mediadevices` reconoce:
    ```bash
    ./webrtc-streamer --list-devices
    ```
    Anote el `Label` (preferiblemente) o el `DeviceID` del dispositivo deseado.

*   **Ejecutar el Servidor:**
    Usa los flags `-v` (para video) y/o `-a` (para audio) con los identificadores obtenidos.
    ```bash
    # Ejemplo usando Labels
    ./webrtc-streamer -v "LabelDeTuCamara" -a "LabelDeTuMicrofono"

    # Ejemplo usando un Label que podría ser hexadecimal (OBS) y otro común
    ./webrtc-streamer -v "video0;video0" -a "obs_virtual_output.monitor" 

    # Ejemplo solo video
    ./webrtc-streamer -v "LabelDeTuCamara"
    ```
    El servidor se iniciará y esperará conexiones en `http://localhost:8080`.

*   **Acceder desde el Cliente:**
    Abre un navegador web y navega a `http://localhost:8080`. La página `client.html` se cargará, y el stream debería comenzar. Múltiples pestañas/navegadores pueden conectarse.

**7. Dependencias Externas**

*   Compilador de Go.
*   Conexión a Internet (para el servidor STUN y la comunicación).
*   Navegador web moderno con soporte WebRTC en los clientes.
*   (Opcional, Linux) `v4l-utils`: Para que el usuario pueda verificar los dispositivos `/dev/videoX` a nivel de sistema, aunque el programa usa su propia enumeración.

**8. Limitaciones y Supuestos**

*   **Fuente Única Compartida:** Todos los clientes reciben el mismo stream. No hay personalización del stream por cliente.
*   **Sin TURN:** La conexión puede fallar en redes con NATs simétricos o firewalls estrictos.
*   **Señalización WebSocket Simple:** No incluye características avanzadas como autenticación, salas, etc.
*   **Robustez de Captura Limitada:** Intenta reintentos iniciales para la captura de medios. No maneja dinámicamente la desconexión/reconexión de la fuente de medios una vez que el servidor está en marcha sin reiniciar el servidor.
*   **Estabilidad de `DeviceID` y `Label`:** La identificación de dispositivos depende de la consistencia proporcionada por `mediadevices` y los drivers del sistema. El uso de `Label` (con fallback a `DeviceID`) intenta mitigar la volatilidad de `DeviceID`.

---
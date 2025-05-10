# webrtc-streamer
Linux Binary for stream system video and audio with webrtc

---

```markdown
# Go WebRTC Multi-Client Streamer

Este proyecto implementa un servidor de streaming multimedia en Go. Captura audio y/o video desde dispositivos seleccionados (cámaras, micrófonos, OBS Virtual Cam) y los transmite en tiempo real a múltiples clientes web utilizando WebRTC. La señalización se maneja a través de WebSockets.

## Características

*   **Streaming de Video y Audio:** Soporta transmisión de video (VP8) y/o audio (Opus).
*   **Selección de Dispositivos por Flags:** Permite especificar dispositivos de entrada por `Label` (nombre) o `DeviceID` vía línea de comandos.
*   **Descubrimiento de Dispositivos:** Flag `--list-devices` para enumerar los dispositivos multimedia detectados por `pion/mediadevices`.
*   **Soporte Multicliente:** Múltiples espectadores pueden conectarse simultáneamente al mismo stream.
*   **Servidor Web Integrado:** Sirve un cliente HTML/JavaScript (`client.html`) para recibir el stream.
*   **Basado en Pion:** Utiliza `pion/webrtc` y `pion/mediadevices`.
*   **Estructura Modular:** Código organizado en componentes (configuración, media, webrtc, servidor).
*   **Reintentos de Captura Inicial:** Intenta capturar los medios varias veces al inicio si la fuente no está disponible inmediatamente.

## Pila Tecnológica

*   **Backend:** Go
    *   WebRTC: `pion/webrtc v4`
    *   Media Devices & Codecs: `pion/mediadevices` (VP8, Opus)
    *   WebSocket: `gorilla/websocket`
    *   HTTP Server: `net/http` (Go standard library)
*   **Frontend (Cliente de Ejemplo):** HTML5, JavaScript (Web API: `RTCPeerConnection`, `WebSocket`, `MediaStream`)
*   **NAT Traversal:** STUN (usando `stun:stun.l.google.com:19302`)

## Requisitos Previos

*   **Go:** Versión 1.18 o superior.
*   **Git:** Para clonar el repositorio.
*   **(Opcional, para Linux):** `v4l-utils` (ej. `sudo apt install v4l-utils`) para inspeccionar dispositivos de video del sistema (`/dev/videoX`). No es estrictamente necesario para el funcionamiento del programa si se utiliza `--list-devices`.

## Instalación y Compilación

1.  **Clonar el Repositorio:**
    ```bash
    git clone https://github.com/TU_USUARIO_GITHUB/TU_REPOSITORIO.git
    cd TU_REPOSITORIO
    ```
    *(Reemplaza `TU_USUARIO_GITHUB/TU_REPOSITORIO` con la URL de tu repositorio)*

2.  **Descargar Dependencias:**
    (Navega al directorio del proyecto si no lo has hecho ya)
    ```bash
    go mod tidy
    ```

3.  **Compilar el Servidor:**
    ```bash
    go build -o webrtc-streamer .
    ```
    Esto creará un ejecutable llamado `webrtc-streamer`.

## Uso

### 1. Listar Dispositivos Disponibles

Para saber qué identificadores (`Label` o `DeviceID`) usar, ejecuta:
```bash
./webrtc-streamer --list-devices
```
Esto mostrará los dispositivos detectados por `pion/mediadevices`, por ejemplo:
```
Dispositivos multimedia disponibles:
  - Tipo: VideoInput, ID: 'xxxxxxxx-xxxx', Label: 'Nombre de tu Cámara (o video0;video0)'
  - Tipo: AudioInput, ID: 'yyyyyyyy-yyyy', Label: 'Nombre de tu Micrófono (o obs_virtual_output.monitor)'
  ...
```
Anota el `Label` (generalmente más fácil) o el `DeviceID` del dispositivo(s) que deseas usar.

### 2. Iniciar el Servidor de Streaming

Usa los flags `-v` para video y/o `-a` para audio, seguidos del `Label` o `DeviceID`.

**Ejemplos:**

*   **Transmitir Video y Audio (usando Labels):**
    ```bash
    ./webrtc-streamer -v "Nombre de tu Cámara" -a "Nombre de tu Micrófono"
    ```
    _Si usas OBS Virtual Camera (que en Linux podría tener el label `video0;video0`) y su monitor de audio:_
    ```bash
    ./webrtc-streamer -v "video0;video0" -a "obs_virtual_output.monitor"
    ```

*   **Transmitir Solo Video:**
    ```bash
    ./webrtc-streamer -v "Nombre de tu Cámara"
    ```

*   **Transmitir Solo Audio:**
    ```bash
    ./webrtc-streamer -a "Nombre de tu Micrófono"
    ```

El servidor se iniciará y esperará conexiones en `http://localhost:8080`. Si una fuente de medios no está disponible inmediatamente, el servidor intentará capturarla varias veces antes de fallar.

### 3. Ver el Stream

Abre tu navegador web y navega a:
`http://localhost:8080`

La página `client.html` se cargará. El video no comenzará automáticamente; deberás usar los controles del reproductor para iniciar la reproducción. Múltiples clientes pueden conectarse.

## Estructura del Proyecto

El código está organizado en los siguientes archivos principales:

*   `main.go`: Punto de entrada, orquestación de componentes.
*   `config.go`: Manejo de flags y configuración.
*   `media_manager.go`: Lógica para la captura y gestión de los streams de medios.
*   `webrtc_manager.go`: Configuración del motor WebRTC de Pion.
*   `server.go`: Implementación del servidor HTTP, manejo de WebSockets y clientes WebRTC.
*   `client.html`: Página HTML del cliente para recibir el stream.
*   `go.mod`, `go.sum`: Gestión de dependencias de Go.

## Limitaciones Conocidas

*   **Fuente Única Compartida:** Todos los clientes reciben el mismo stream.
*   **Sin Servidor TURN:** La conexión puede fallar en redes complejas si STUN no es suficiente.
*   **Señalización Simple:** No incluye características avanzadas como autenticación o salas.
*   **Robustez de Captura Limitada:** Incluye reintentos iniciales. No maneja dinámicamente la desconexión/reconexión de la fuente de medios una vez que el servidor está en marcha sin reiniciar el servidor (el stream se detendría si la fuente se pierde).

## Licencia

Este proyecto está bajo la Licencia [NOMBRE DE TU LICENCIA AQUÍ, ej. MIT]. Ver el archivo `LICENSE` para más detalles.
```

---

**Recordatorios antes de subir a GitHub:**

1.  **Reemplaza los placeholders:**
    *   `TU_USUARIO_GITHUB/TU_REPOSITORIO` con la URL real de tu repositorio.
    *   `[NOMBRE DE TU LICENCIA AQUÍ, ej. MIT]` con la licencia que elijas.
2.  **Crea el archivo `LICENSE`** con el texto completo de la licencia elegida.
3.  **Crea un archivo `.gitignore`** (como se discutió en una respuesta anterior, para ignorar binarios, etc.).
    Ejemplo básico:
    ```
    # Binarios
    webrtc-streamer
    *.exe
    *.out

    # Archivos de IDE/Editor (ejemplos)
    .vscode/
    .idea/
    *.swp
    *~
    ```
4.  Asegúrate de que los archivos `go.mod` y `go.sum` estén presentes y actualizados (`go mod tidy`).
5.  Asegúrate de que el archivo `client.html` que usas es la versión que quieres subir.

Este `README.md` está más enfocado en cómo un usuario o desarrollador puede empezar a usar y entender tu proyecto rápidamente. ¡Mucha suerte con la subida a GitHub!

# Especificación: Conexión VPN Principal (OpenVPN 3)

## 1. Visión General
Este módulo central gestiona la conexión hacia la red privada virtual (VPN) utilizando el cliente de línea de comandos `openvpn3`. Engloba la inicialización del entorno (dispositivos de red y demonios del sistema), la gestión de perfiles inyectados, la ejecución y control de la sesión, y la detección del flujo de autenticación externa (Single Sign-On - SSO).

## 2. Objetivos
- Automatizar la preparación del sistema operativo (o contenedor) para soportar interfaces de red virtuales (TUN) y el bus de mensajes del sistema (D-Bus), dependencias estrictas de `openvpn3`.
- Gestionar el ciclo de vida de la sesión VPN (iniciar, detener, verificar estado).
- Detectar automáticamente cuando el servidor de VPN requiere autorización basada en la web (ej. SAML) y extraer la URL de SSO para exponerla al cliente (frontend).

## 3. Inicialización y Requerimientos del Sistema (`startSystemServices`)
Antes de iniciar cualquier conexión, la aplicación garantiza que el entorno operativo esté preparado:
- **Dispositivo TUN**: Si no se encuentra `/dev/net/tun`, la aplicación crea el nodo de dispositivo ejecutando `mknod c 10 200` usando `sudo`.
- **Demonio D-Bus**: Se asegura la existencia de `/run/dbus`, se genera un UUID y se arranca el demonio mediante `dbus-daemon --system --fork`. Esto es crítico para la comunicación interna de `openvpn3`.

## 4. Ingesta y Gestión del Perfil OpenVPN (`initVPN`)
- La configuración para el cliente VPN se inyecta mediante la variable de entorno `OPENVPN_PROFILE`.
- Esta variable debe contener el archivo `.ovpn` codificado en formato **Base64**.
- La aplicación decodifica el perfil en tiempo de ejecución y lo almacena físicamente en `/tmp/vpn/servipar.ovpn` para ser consumido por el CLI de openvpn.

## 5. Diseño de API

### 5.1. `POST /api/start`
- **Descripción**: Inicia el proceso de conexión de la VPN.
- **Flujo de Ejecución**:
  1. Escribe el perfil decodificado a disco.
  2. Lanza el comando `sudo openvpn3 session-start --config /tmp/vpn/servipar.ovpn` y captura toda su salida (stdout/stderr).
  3. Despliega una *goroutine* (hilo secundario) que espera (haciendo polling de 1 segundo) a que aparezca la interfaz de red `tun0`. Al detectarla, ejecuta `openvpn3 log` enviando la salida al stdout de la aplicación para fines de depuración.
  4. Analiza la salida capturada del comando de inicio utilizando una expresión regular (`https://[^\s]+`) para buscar enlaces de autenticación.
- **Respuestas (JSON)**:
  - **Requiere Autenticación SSO**: Si se encuentra una URL, devuelve `{"status": "auth_required", "url": "https://..."}`. El cliente debe visitar esa URL.
  - **Conectado con Éxito**: Si no hay URL y la interfaz `tun0` está activa, devuelve `{"status": "connected"}`.
  - **Error**: Si falla y no hay `tun0`, devuelve `{"error": "...", "logs": "..."}` con la salida del comando.

### 5.2. `POST /api/stop`
- **Descripción**: Fuerza la terminación de la sesión VPN activa.
- **Flujo de Ejecución**: Lanza el comando `sudo openvpn3 session-manage --disconnect --config /tmp/vpn/servipar.ovpn`.
- **Respuestas**: Siempre devuelve `{"status": "disconnected"}` tras ejecutar el comando.

### 5.3. `GET /api/status`
- **Descripción**: Verifica el estado actual y real de la conexión de red.
- **Flujo de Ejecución**: En lugar de depender del estado del proceso, inspecciona directamente las interfaces de red del sistema operativo usando el paquete `net` de Go. Si existe una interfaz llamada `tun0`, se asume conexión.
- **Respuestas**: Devuelve un booleano indicando el estado `{"connected": true | false}`.

## 6. Consideraciones para la Integración con UI (`/web`)
- Cuando la interfaz gráfica llama a `/api/start`, debe estar preparada para recibir un estado `auth_required`. Al recibirlo, debe abrir una ventana, iframe o mostrar un enlace interactivo al usuario para que haga click en la `url` provista.
- El polling o refresco de estado debe consultar el endpoint `/api/status`.

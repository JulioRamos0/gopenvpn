# Especificación: Diseño de Arquitectura y Estructura del Proyecto

## 1. Visión General
Esta especificación define la nueva estructura de directorios y la arquitectura del código fuente para la aplicación `gopenvpn`. El objetivo es adoptar un diseño idiomático basado en el estándar de la comunidad de Go (Standard Go Project Layout). Esto mejorará la mantenibilidad, escalabilidad y la separación de responsabilidades, facilitando la migración desde la estructura actual (basada en el archivo único `cmd/server/main.go`).

## 2. Estructura de Directorios

```text
gopenvpn/
├── cmd/
│   └── server/
│       └── main.go           # Punto de entrada de la aplicación
├── config/                   # Carga, validación y gestión de variables de entorno
├── pkg/                      # Código público que puede ser exportado o utilizado por otros módulos
├── api/                      # Contratos (OpenAPI/Swagger specs), archivos proto y handlers HTTP
├── web/                      # Componentes frontend, assets, templates HTML (si no es SPA)
├── go.mod                    # Definición del módulo en la raíz
└── go.sum                    # Checksums de dependencias
```

## 3. Responsabilidades por Componente

### 3.1. `/cmd/server/main.go`
- **Responsabilidad**: Es el archivo principal que se compila para generar el binario del servidor.
- **Dominio**: Se encarga únicamente del "wiring" (conexión de dependencias). Llama al inicializador de configuración (`/config`), configura las rutas HTTP (`/api`), inicializa los servicios del sistema y levanta el servidor web.
- **Restricciones**: No debe contener lógica de negocio directa.

### 3.2. `/config/`
- **Responsabilidad**: Centralizar el acceso a la configuración externa.
- **Dominio**: Lectura de variables de entorno (`PORT`, `OPENVPN_PROFILE`), validación (ej. asegurar que el puerto sea `>= 1024`) y provisión de estructuras de configuración (ej. `type AppConfig struct`) fuertemente tipadas para el resto de la aplicación.

### 3.3. `/pkg/`
- **Responsabilidad**: Código de biblioteca y lógica de dominio (Core) de la aplicación que podría ser reutilizable.
- **Dominio**: 
  - **vpn**: Lógica de interacción con `openvpn3`, rutinas de monitoreo de la interfaz `tun0` y análisis de URLs (SSO).
  - **proxy**: Gestión y persistencia del proxy inverso, lectura/escritura de `/data/proxies.json` e interceptación de tráfico HTTP.

### 3.4. `/api/`
- **Responsabilidad**: Capa de transporte y definición de contratos.
- **Dominio**: Contiene los controladores (Handlers) HTTP que reciben las peticiones de los clientes, validan la entrada (JSON payloads) y llaman a los servicios en `/pkg`. Además, es el lugar ideal para alojar documentación de APIs como esquemas Swagger/OpenAPI o definiciones de gRPC en caso de ser necesarias en el futuro.

### 3.5. `/web/`
- **Responsabilidad**: Presentación y UI.
- **Dominio**: Almacena los recursos estáticos como `index.html`, archivos CSS, JS e imágenes. Contiene la lógica en Go (si aplica) para usar `html/template` o directivas `//go:embed` para incluir los assets en el binario final. Reemplazará al actual directorio `/web`.

## 4. Estrategia de Migración (Propuesta)
Una vez aprobada esta especificación, los pasos lógicos para refactorizar el código actual serán:
1. **Inicializar Módulo**: Ejecutar `go mod init` en el directorio raíz de `gopenvpn` (y eliminar el interno de `web` si existiera independientemente).
2. **Crear Directorios**: Generar el andamiaje (`mkdir -p cmd/server config pkg/vpn pkg/proxy api web`).
3. **Migrar UI**: Mover `index.html` a `web/`.
4. **Extraer Lógica**: Desmembrar secuencialmente las partes de `cmd/server/main.go` hacia sus respectivos paquetes en `/config`, `/pkg/vpn`, `/pkg/proxy` y `/api`.
5. **Reconstruir Main**: Crear el nuevo `cmd/server/main.go` uniendo las piezas.

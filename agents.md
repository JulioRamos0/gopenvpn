# Contexto del Proyecto y Reglas

Este archivo contiene las directrices generales para el desarrollo en este repositorio.

## Estructura del Proyecto (Standard Go Layout)
- **cmd/server**: Punto de entrada de la aplicación Go.
- **pkg/**: Paquetes core de la aplicación separados por dominio de negocio:
  - `vpn`: Integración y monitoreo del proceso OpenVPN 3 y D-Bus.
  - `proxy`: Servidor reverse proxy TCP empotrado que envía puertos locales a la red interna.
  - `tunnel`: Gestor de túneles SSH y AWS SSM (Systems Manager) que encapsula comandos subyacentes.
- **api/**: Manejadores de las rutas HTTP expuestas por el servidor.
- **web/**: Interfaz visual nativa en Vanilla JS, CSS (estilo Glassmorphism) y HTML estático servido por `//go:embed`.
- **data/**: Almacenamiento persistente en formato JSON mapeado en volúmenes Docker (para `proxies.json`, `tunnels.json` y llaves SSH).

## Metodología de Trabajo
- Trabajaremos bajo el enfoque de **Spec-Driven Development** (Desarrollo Guiado por Especificaciones).
- Esto implica que las funcionalidades y cambios deben estar definidos por una especificación antes o durante su implementación.
- **Ciclo de Vida de Cambios**: Ante cualquier solicitud de cambio, el primer paso es identificar si existe una especificación (`spec`) relacionada en el directorio `/specs`. Si existe, después de que el usuario apruebe el plan de implementación y se manipule el código, **es obligatorio actualizar la especificación** para asegurar que se mantenga sincronizada con el código real.

## Gestión de Especificaciones
- Todas las especificaciones del proyecto se administrarán y mantendrán dentro del directorio `/specs`.

## Reglas de Privacidad y Ejemplos
- **Dominios Ficticios**: NUNCA generar comentarios, documentación o código de ejemplo que utilicen dominios reales provenientes de variables de entorno o configuraciones. Siempre se deberán utilizar dominios genéricos y ficticios para los ejemplos (por ejemplo: `corp.aws.sso`, `git.corp.com`, `hr.corp.com`).

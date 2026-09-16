# Especificación: Administración de Proxy

## 1. Visión General
El módulo de Administración de Proxy en GOpenVPN se encarga de gestionar reglas de reverse proxy (proxy inverso). Permite mapear puertos locales a URLs de destino (targets), facilitando el acceso a servicios (por ejemplo, aquellos detrás de la VPN) desde el exterior de manera controlada.

## 2. Objetivos
- Proveer una API HTTP para configurar (crear, listar, actualizar y eliminar) reglas de proxy inverso.
- Persistir la configuración de las reglas para que se restauren automáticamente al reiniciar la aplicación.
- Redirigir el tráfico HTTP/HTTPS de manera transparente, ajustando cabeceras relevantes para evitar fugas de información interna y corregir problemas de enrutamiento (ej. reescritura de redirects y cookies).

## 3. Modelo de Datos
Una regla de proxy se define mediante la siguiente estructura de datos (`ProxyRule`):
- **Port** (`int`): El puerto local en el que el servidor proxy escuchará. Por seguridad, se requiere que sea `>= 1024`.
- **Target** (`string`): La URL de destino a la cual se reenviará el tráfico (ej. `http://10.8.0.5:80`).

## 4. Requerimientos Funcionales

### 4.1. Gestión y Persistencia
- Las reglas de proxy se mantienen en memoria y se sincronizan (guardan/cargan) de forma persistente en un archivo JSON ubicado en `/data/proxies.json`.
- Al iniciar la aplicación, el sistema lee automáticamente el archivo de configuración y arranca todos los servidores proxy previamente guardados.

### 4.2. Comportamiento del Reverse Proxy
Para cada regla activa, se levanta un servidor HTTP independiente utilizando un `SingleHostReverseProxy`. El proxy modifica el tráfico al vuelo aplicando las siguientes reglas:
- **Modificación de la Petición (Request)**:
  - Cambia el `Host` por el del target configurado.
  - Preserva el host original en la cabecera `X-Original-Host`.
  - Elimina cabeceras que puedan filtrar la IP interna de la red de Docker o del host (`X-Real-IP`, `X-Forwarded-For`).
  - Ajusta dinámicamente `X-Forwarded-Proto` (HTTP o HTTPS) según corresponda.
- **Modificación de la Respuesta (Response)**:
  - **Redirects**: Si el servidor destino devuelve una cabecera `Location` (redirección) apuntando a su propio host, el proxy la reescribe para que el cliente apunte al host original (`X-Original-Host`).
  - **Cookies**: Se interceptan las cabeceras `Set-Cookie` y se elimina el atributo `Domain=` para prevenir problemas de aceptación de cookies en el navegador del cliente debido a discrepancias de dominio.
- **TLS**: Ignora las validaciones de certificados TLS (InsecureSkipVerify: true) para facilitar la conexión con servicios internos con certificados autofirmados.

### 4.3. Control de Ciclo de Vida
- Crear o actualizar una regla arranca o reinicia el servidor HTTP de forma asíncrona (goroutine) en el puerto especificado.
- Eliminar una regla invoca el cierre (`Close()`) del servidor HTTP asociado y lo remueve del mapeo de procesos activos.

## 5. Diseño de API (`/api/proxies`)
La aplicación expone un endpoint principal multiplexado por el método HTTP:

- `GET /api/proxies`
  - **Descripción**: Devuelve la lista de todas las reglas de proxy actuales.
  - **Respuesta Exitosa**: `200 OK`, con un Array JSON de objetos `ProxyRule`.

- `POST /api/proxies`
  - **Descripción**: Añade una nueva regla o actualiza el target de un puerto existente. Aplica e inicia el servidor inmediatamente.
  - **Cuerpo Esperado (JSON)**: `{ "port": 8080, "target": "http://10.8.0.2" }`
  - **Validaciones**: Retorna `400 Bad Request` si el puerto es `< 1024` o si el target está vacío.

- `DELETE /api/proxies?port={port}`
  - **Descripción**: Detiene el proxy en el puerto indicado y elimina la regla de la configuración.
  - **Parámetros**: El puerto debe pasarse en la query string (`?port=...`).
  - **Respuesta**: `200 OK` en caso de éxito. Retorna `404 Not Found` si el puerto no existía y `400 Bad Request` si el puerto es inválido.

## 6. Consideraciones para la UI (`/web`)
- Se requiere un panel que consulte (`GET`) y muestre la lista de puertos y URLs configuradas de forma amigable.
- Un formulario para agregar nuevas reglas, enviando la información mediante un `POST`. El formulario en el frontend debe prevenir o advertir al usuario sobre el uso de puertos reservados (< 1024).
- Un botón por cada regla para eliminarla, que dispare una petición `DELETE` con el puerto correspondiente.

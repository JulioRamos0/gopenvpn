# Gestión Visual de Túneles (AWS SSM & SSH)

## 1. Descripción General
El gestor de túneles es un submódulo integrado en la Interfaz Web y el servidor Go. Permite a los usuarios administrar (crear, editar, eliminar) y monitorear túneles (port-forwarding locales hacia la nube) en tiempo real, operando como una pestaña dedicada dentro de la interfaz gráfica principal.

## 2. Autenticación y Credenciales (AWS)
- Las credenciales deben inyectarse en el entorno a través de la variable `AWS_CREDENTIALS` codificada en Base64.
- Al decodificarse, esta variable representa el contenido completo de un archivo estándar de credenciales de AWS (con múltiples perfiles como `[default]`, `[perfil1]`).
- Cuando se registre un túnel de tipo AWS SSM, el usuario podrá indicar el **nombre del perfil** a utilizar. Si no se especifica, el servidor Go intentará utilizar el perfil `[default]`.

## 3. Persistencia (`/data/tunnels.json`)
El estado y definición de los túneles se persistirá de manera local en `data/tunnels.json`.

**Esquema de Ejemplo:**
```json
[
  {
    "id": "1",
    "name": "Database RDS (SSM)",
    "type": "aws-ssm",
    "aws_profile": "perfil1",
    "target_tags": {"Name": "*tunnel*"},
    "remote_host": "db.corp.aws.internal",
    "remote_port": 5432,
    "local_port": 15432
  },
  {
    "id": "2",
    "name": "Cache Redis (SSH)",
    "type": "ssh",
    "jump_host": "bastion.corp.com",
    "jump_user": "ec2-user",
    "remote_host": "redis.corp.aws.internal",
    "remote_port": 6379,
    "local_port": 16379
  }
]
```

## 4. Endpoints API (Go Backend)
- **`GET /api/tunnels`**: Lista todos los túneles y su estado en memoria (`stopped`, `connected`, `error`).
- **`POST /api/tunnels`**: Crea un nuevo túnel en `data/tunnels.json`.
- **`PUT /api/tunnels/:id`**: Actualiza los datos de un túnel existente.
- **`DELETE /api/tunnels/:id`**: Elimina un túnel.
- **`POST /api/tunnels/:id/start`**: Inicia el subproceso SSM o la conexión SSH.
- **`POST /api/tunnels/:id/stop`**: Apaga y limpia los subprocesos del túnel seleccionado.

## 5. Integración Interfaz Web (UI)
La UI en `web/index.html` tendrá:
1. **Navegación:** Pestañas (Tabs) para alternar entre "Proxys" y "Tunnels".
2. **Panel Principal:** 
   - Lista dinámica de los túneles actuales.
   - Indicadores de estado visuales y botones "Conectar/Desconectar".
3. **Modal de Gestión:**
   - Un modal interactivo para **Alta y Edición** de túneles.
   - Dependiendo del `Tipo` seleccionado (SSM o SSH), el formulario mostrará dinámicamente los campos pertinentes (Ej: campo "AWS Profile" y "Target Tags" sólo si es SSM).

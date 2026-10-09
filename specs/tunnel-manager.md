# Gestión Visual de Túneles (AWS SSM & SSH)

## 1. Descripción General
El gestor de túneles es un submódulo integrado en la Interfaz Web y el servidor Go. Permite a los usuarios administrar (crear, editar, eliminar) y monitorear túneles (port-forwarding locales hacia la nube) en tiempo real, operando como una pestaña dedicada dentro de la interfaz gráfica principal.

## 2. Autenticación y Perfiles AWS (Gestión 100% desde la UI)
Se ha descontinuado totalmente el uso de la variable de entorno `AWS_CREDENTIALS`. Toda la configuración y autenticación de AWS se gestiona de forma interactiva y persistente a través de la Interfaz Web.

Al configurar un túnel de tipo **AWS SSM**, el usuario define:
- **`is_sso` (booleano):** Indica si el perfil utiliza autenticación federada AWS IAM Identity Center (SSO).

### A. Si es SSO (`is_sso: true`)
Se solicitan los siguientes datos para configurar el perfil en `~/.aws/config`:
- **`sso_start_url`**: URL del portal SSO (ej. `https://my-sso-portal.awsapps.com/start`).
- **`sso_region`**: Región del servicio SSO (ej. `us-east-1`).
- **`sso_account_id`**: ID de la cuenta de AWS (12 dígitos).
- **`sso_role_name`**: Nombre del rol de permisos asignado (ej. `AdministratorAccess`, `DeveloperRole`).
- **`region`**: Región destino donde se ejecuta la instancia EC2/SSM (ej. `us-east-1`).

> **Nota técnica sobre credenciales:** Cuando se utiliza SSO, **NO** se requieren ni se deben solicitar `aws_access_key_id` ni `aws_secret_access_key`. El acceso se gestiona a través del flujo Device Authorization Grant de AWS SSO (`aws sso login`).

### B. Si NO es SSO (`is_sso: false`)
Se configuran credenciales IAM clásicas o temporales:
- **`region`**: Región destino de AWS.
- **`aws_access_key_id`**: ID de clave de acceso.
- **`aws_secret_access_key`**: Clave de acceso secreta.
- **`aws_session_token`** (Opcional): Requerido si son credenciales temporales generadas por STS/MFA.

---

## 3. Persistencia (`/data/tunnels.json`)
El estado y definición de los túneles se persistirá de manera local en `data/tunnels.json`.

**Esquema de Ejemplo:**
```json
[
  {
    "id": "1",
    "name": "Database RDS (SSM con SSO)",
    "type": "aws-ssm",
    "is_sso": true,
    "aws_profile": "corp-dev-sso",
    "sso_start_url": "https://corp.awsapps.com/start",
    "sso_region": "us-east-1",
    "sso_account_id": "123456789012",
    "sso_role_name": "DeveloperRole",
    "region": "us-east-1",
    "target": "i-0123456789abcdef0",
    "remote_host": "db.corp.aws.internal",
    "remote_port": 5432,
    "local_port": 15432
  },
  {
    "id": "2",
    "name": "Analytics DB (SSM con Keys Estáticas)",
    "type": "aws-ssm",
    "is_sso": false,
    "aws_profile": "corp-iam-user",
    "region": "us-west-2",
    "aws_access_key_id": "AKIAIOSFODNN7EXAMPLE",
    "aws_secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
    "target": "i-0987654321fedcba0",
    "remote_host": "analytics.corp.internal",
    "remote_port": 3306,
    "local_port": 13306
  },
  {
    "id": "3",
    "name": "Cache Redis (SSH)",
    "type": "ssh",
    "jump_host": "bastion.corp.com",
    "jump_user": "ec2-user",
    "ssh_key_file": "id_rsa",
    "remote_host": "redis.corp.aws.internal",
    "remote_port": 6379,
    "local_port": 16379
  }
]
```

---

## 4. Endpoints API (Go Backend)
- **`GET /api/tunnels`**: Lista todos los túneles y su estado en memoria (`stopped`, `connected`, `needs_login`, `authenticating`, `error`).
  - Para túneles SSO, el backend valida periódicamente o on-demand la vigencia del token (`aws sts get-caller-identity --profile <perfil>`).
  - Si el token expiró o no existe sesión activa, expone `sso_authenticated: false` y el túnel pasa a estado `needs_login`.
- **`POST /api/tunnels`**: Crea un nuevo túnel en `data/tunnels.json` y genera/actualiza la configuración del perfil AWS en `~/.aws/config`.
- **`PUT /api/tunnels/:id`**: Actualiza los datos de un túnel existente.
- **`DELETE /api/tunnels/:id`**: Elimina un túnel.
- **`GET /api/tunnels/:id/sso-status`**: Valida explícitamente si el perfil de AWS SSO asociado cuenta con credenciales activas y no expiradas. Devuelve:
  ```json
  {
    "authenticated": false,
    "auth_url": "https://device.sso.us-east-1.amazonaws.com/",
    "user_code": "ABCD-WXYZ",
    "expires_in": 300
  }
  ```
- **`POST /api/tunnels/:id/sso-login`**: Inicia el proceso de autenticación ejecutando en segundo plano:
  `aws sso login --profile <perfil> --no-browser`
  - Intercepta en `stdout` la URL de verificación (`auth_url`) y el código del dispositivo (`user_code`).
  - Cambia el estado a `authenticating`.
  - El backend mantiene un worker o listener que espera a que el usuario complete la autorización en el navegador para marcar la sesión como `sso_authenticated: true`.
- **`POST /api/tunnels/:id/start`**: Inicia el subproceso SSM o la conexión SSH.
  - Si es de tipo SSO y `sso_authenticated == false`, rechaza el inicio directo e instruye a completar el login SSO primero o dispara el flujo de login.
- **`POST /api/tunnels/:id/stop`**: Apaga y limpia los subprocesos del túnel seleccionado.

---

## 5. Integración Interfaz Web (UI)
La UI en `web/index.html` y `web/app.js` tendrá:
1. **Navegación:** Pestañas (Tabs) para alternar entre "Proxys" y "Tunnels".
2. **Panel Principal (Lista de Túneles):** 
   - Cada tarjeta/fila de túnel mostrará su estado actual:
     - 🟢 **Conectado (`connected`)**: Túnel activo y canal SSM/SSH abierto.
     - ⚪ **Detenido (`stopped`)**: Listo para conectar (si está autenticado).
     - 🟠 **Autenticación Requerida (`needs_login`)**: Aplica a túneles SSO sin sesión activa.
     - 🔵 **Esperando Autorización (`authenticating`)**: Login SSO en curso con URL y código visible.
     - 🔴 **Error (`error`)**: Detalle del error.
   
3. **Mecanismo Visual de Autenticación SSO (Similar al flujo VPN):**
   - **Indicador de Autenticación:** Badge visual `SSO Autenticado` o `SSO Expirado / No Autenticado`.
   - **Botón de Acción Contextual:**
     - Si el túnel está **autenticado** (o no requiere SSO): Se muestra el botón tradicional **"Conectar"** / **"Desconectar"**.
     - Si el túnel **NO está autenticado**:
       - En lugar de "Conectar", se habilita un botón prominente: **"🔑 Autenticar AWS SSO"** (con comportamiento reactivo idéntico al de conectar VPN).
       - Al hacer clic, se llama a `POST /api/tunnels/:id/sso-login`.
       - La tarjeta muestra un bloque interactivo con:
         - El enlace para abrir la consola de SSO: `Abrir enlace de verificación ↗`.
         - El código del dispositivo para copiar con un clic: `Código: ABCD-WXYZ`.
         - Un indicador de carga o spinner: *"Esperando confirmación en el navegador..."*.
       - Una vez completado el flujo en AWS, el estado cambia automáticamente a `Autenticado` y el botón se transforma en **"Conectar"**.

4. **Modal de Gestión (Wizard de 2 Etapas):**
   Para ofrecer una interfaz limpia y libre de sobrecarga cognitiva, el formulario se divide en dos fases progresivas:

   - **Etapa 1: Datos Básicos**
     - `Nombre del Túnel` / Identificador.
     - `Puerto Local` & `Puerto Remoto`.
     - `Remote Host / IP Destino`.
     - Botón **"Siguiente ➔"** con validación de campos obligatorios.

   - **Etapa 2: Conexión y Autenticación**
     - Selector de `Tipo de Conexión`: `AWS SSM` o `SSH`.
     - **Si es AWS SSM:**
       1. **Resolución de Target EC2:**
          - Selector Radio: `Target ID directo` o `Filtro por Tags`.
          - Se muestra únicamente el campo correspondiente (`EC2 Instance Target ID` o `Tag Filter`).
       2. **Selección de Perfil AWS:**
          - Dropdown con los **perfiles previamente dados de alta** (reutilización inmediata sin reintroducir claves) + opción **"➕ Configurar un perfil nuevo..."**.
       3. **Si se elige "Configurar un perfil nuevo":**
          - Campo: `Nombre del Nuevo Perfil`.
          - Toggle: `¿Es perfil de AWS SSO (IAM Identity Center)?`.
          - **Si es SSO:** Solicita únicamente `SSO Start URL`, `SSO Region`, `Destination Region`, `SSO Account ID` y `SSO Role Name` (oculta claves de acceso).
          - **Si NO es SSO:** Solicita `AWS Region`, `AWS Access Key ID` y `AWS Secret Access Key`.
     - **Si es SSH:**
       - Solicita `Jump Host`, `Jump User` y `SSH Key Filename`.
     - Botones de navegación: **"⬅ Atrás"** y **"Guardar Túnel 💾"**.

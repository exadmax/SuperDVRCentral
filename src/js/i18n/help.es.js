(function (CD) {
  // Manual de Config → Ayuda en español. Mismas secciones (ids) que help.pt.js; los nombres de botones y pestañas deben coincidir con los de es.js.
  const tip = t => '<div class="help-tip"><b>Consejo:</b> ' + t + '</div>';
  const warn = t => '<div class="help-warn"><b>Atención:</b> ' + t + '</div>';

  CD.i18n.addHelp('es', [
    { id: 'comecar', title: 'Cómo funciona Config', html: `
      <p>Config es donde se ajusta todo lo que antes estaba escondido en archivos: cámaras, agendas, tráfico, menú, ciudad del clima, festivos y las cuentas de integración. Solo funciona <b>abierta con la dirección del servidor de la casa</b> (el hub), porque es el hub quien guarda la configuración.</p>
      <h3>Las pestañas</h3>
      <table class="help-t"><tr><th>Pestaña</th><th>Para qué sirve</th></tr>
        <tr><td>Servidor</td><td>Uso de CPU y memoria, y los botones para liberar memoria.</td></tr>
        <tr><td>Cámaras</td><td>Alta de las cámaras, grabación, carpeta de las grabaciones y envío a Google Drive.</td></tr>
        <tr><td>Agendas y rutas</td><td>Agendas (iCal) y el tráfico de cada persona.</td></tr>
        <tr><td>Panel</td><td>Ciudad y clima, modo nocturno, rotación de las cámaras, caducidad, lugares del inventario, vídeos y festivos locales.</td></tr>
        <tr><td>Libros</td><td>Activa o desactiva la página Libros y define las categorías y los dueños de los libros.</td></tr>
        <tr><td>Menú</td><td>Los botones de acceso directo del pie de la pantalla de inicio.</td></tr>
        <tr><td>Integraciones</td><td>Hoja de cálculo de profesionales, cuentas de juegos, carpeta de vídeos, tiempos de caché y red.</td></tr>
        <tr><td>Idioma</td><td>Idioma de la interfaz: portugués, inglés, español o chino (vale solo en el dispositivo).</td></tr>
        <tr><td>Compatibilidad</td><td>Modo para TV que solo reproducen un vídeo a la vez (vale solo en el dispositivo).</td></tr>
        <tr><td>Ayuda</td><td>Esta página.</td></tr></table>
      <h3>Guardar</h3>
      <ul><li>Cada tarjeta tiene su propio botón <b>Guardar</b>. Guardar una tarjeta no guarda las demás.</li>
        <li>Si el hub rechaza un valor, aparece un mensaje <span class="cfg-err">en rojo</span> en la propia tarjeta y <b>lo que escribió sigue ahí</b> para corregirlo.</li>
        <li>Los botones <b>Quitar</b> actúan al instante (piden confirmación). Las filas nuevas que aún no guardó desaparecen sin hablar con el servidor.</li></ul>
      <h3>Cuándo vale el cambio</h3>
      <table class="help-t"><tr><th>Qué cambió</th><th>Cuándo vale</th></tr>
        <tr><td>Cámaras, agendas, tráfico, menú</td><td>Al instante, en el servidor. Las pantallas ya abiertas actualizan el menú y el panel en un máximo de 5 minutos (o al recargar).</td></tr>
        <tr><td>Pestaña Panel (ciudad, nocturno, rotación, caducidad, lugares, vídeos, festivos)</td><td>Al instante; las pantallas abiertas lo toman en un máximo de 5 minutos.</td></tr>
        <tr><td>Carpeta de grabaciones, Drive, hoja de cálculo, juegos, carpeta de vídeos, caché, IPv4</td><td><b>Solo después de reiniciar el hub.</b> La pantalla muestra una franja amarilla “pendiente de reinicio” con el botón <b>Reiniciar el hub ahora</b>.</td></tr>
        <tr><td>Idioma y modo compatibilidad</td><td>Al instante, solo en este dispositivo (la pantalla se recarga al cambiar el idioma).</td></tr></table>
      <h3>Dónde se guarda</h3>
      <p>Todo lo que edita se guarda en <code>hub.settings.json</code>, en la misma carpeta que <code>hub.config.json</code> en el servidor. El archivo de configuración original sigue valiendo para todo lo que aún no editó. Para <b>devolver una sección al original</b>, borre su clave en <code>hub.settings.json</code> (o el archivo entero) y reinicie el hub. Haga una copia de <code>hub.settings.json</code> como respaldo.</p>
      <h3>Secretos (contraseñas, claves, tokens)</h3>
      <p>Las contraseñas de cámaras, las direcciones secretas de agenda y las claves de API <b>nunca vuelven a la pantalla</b>: aparecen enmascaradas (<code>*****</code>) o como “configurada”. Dejar el campo <b>en blanco significa mantener el valor actual</b>. Para cambiarlo, escriba el valor nuevo. Algunos campos tienen la opción “borrar la configurada”.</p>
      ${warn('el hub está abierto a la red de casa y <b>no tiene inicio de sesión</b>: cualquier persona conectada a su Wi-Fi puede abrir Config. No exponga el hub a internet.')}` },

    { id: 'servidor', title: 'Servidor', html: `
      <h3>Medidores</h3>
      <p>Muestran el uso de CPU y memoria del propio hub y de toda la máquina, actualizados cada 5 segundos. Si la memoria de la máquina está casi llena, el panel puede ir lento: use “Liberar memoria”.</p>
      <h3>Liberar memoria</h3>
      <ul><li><b>Apagar el escritorio</b>: detiene el entorno gráfico del servidor (pantalla, navegador, audio del escritorio). No afecta al hub, al panel ni a las cámaras. Vuelve solo en el próximo reinicio de la máquina.</li>
        <li><b>Desactivar la central</b>: detiene el hub. El panel, las cámaras y la agenda se quedan sin servicio hasta que alguien ejecute en el servidor <code>sudo systemctl start centraldiogo-hub</code>.</li></ul>
      <p>Los dos botones piden <b>dos toques</b>: el primero arma el botón (“¿Confirmar?”, durante 5 segundos) y el segundo lo ejecuta. Solo funcionan en el servidor Linux, con la regla de polkit instalada por <code>install.sh</code>.</p>` },

    { id: 'cameras', title: 'Cámaras: alta', html: `
      <p>En <b>Cámaras → Cámaras</b> están las cámaras que aparecen en la página Cámaras y en la rotación de la pantalla de inicio.</p>
      <h3>Añadir</h3>
      <ol><li>Haga clic en <b>+ Añadir cámara</b>.</li>
        <li><b>Nombre</b>: cómo aparece sobre la imagen (p. ej.: Garaje). También pasa a ser el nombre de los archivos de grabación.</li>
        <li><b>Dirección RTSP</b>: la dirección de vídeo de la cámara (vea los ejemplos abajo).</li>
        <li>Haga clic en <b>Guardar cámaras</b>. La cámara aparece al instante, sin reiniciar.</li></ol>
      <h3>Direcciones RTSP</h3>
      <table class="help-t"><tr><th>Tipo</th><th>Dirección</th></tr>
        <tr><td>Intelbras / Dahua (Mibo Smart, iM)</td><td><code>rtsp://admin:CONTRASEÑA@192.168.0.10:554/cam/realmonitor?channel=1&amp;subtype=1</code></td></tr>
        <tr><td>Cámara sin contraseña (p. ej.: modelos “Hankvision”)</td><td><code>rtsp://192.168.0.11:554/live/mpeg4</code></td></tr></table>
      <ul><li><b>Use el substream (<code>subtype=1</code>)</b>: es H.264, que el hub reenvía al navegador. El <code>subtype=0</code> de muchas cámaras es H.265 y <b>no funciona</b>. El hub avisa “la cámara no entrega H.264 en esta dirección”.</li>
        <li>El usuario suele ser <code>admin</code> y la contraseña es la definida en la app de la cámara (Mibo Smart). Equivocarse varias veces de contraseña bloquea la cámara unos minutos.</li>
        <li><b>Los caracteres especiales de la contraseña</b> van codificados: <code>@</code> = <code>%40</code>, <code>!</code> = <code>%21</code>, <code>#</code> = <code>%23</code>, <code>:</code> = <code>%3A</code>, <code>/</code> = <code>%2F</code>.</li>
        <li>Reserve la IP de la cámara en el router (DHCP fijo). Si la IP cambia, la cámara deja de funcionar hasta que corrija la dirección aquí.</li>
        <li>Sin audio: el vídeo en vivo siempre es mudo.</li></ul>
      <h3>Editar y quitar</h3>
      <ul><li>Para <b>renombrar</b>, cambie el nombre y guarde. Atención: los archivos nuevos pasan a usar el nombre nuevo.</li>
        <li>Para <b>cambiar la contraseña/dirección</b>, escriba la dirección completa nueva. En blanco mantiene la actual (el campo muestra la actual enmascarada).</li>
        <li>El <b>orden</b> de la lista es el orden en la página Cámaras y en la rotación.</li>
        <li><b>Quitar</b> apaga la cámara al instante y cierra el archivo de grabación. Sus grabaciones antiguas se quedan en la carpeta, pero sin dueño: pasan a seguir la retención predeterminada (7 días) y después se borran. Descargue antes lo que quiera conservar.</li>
        <li>Máximo de 16 cámaras.</li></ul>
      ${tip('una cámara <b>nueva</b> empieza a grabar sola si el interruptor general “Grabar las cámaras” está activado. Desactívelo en la tarjeta de grabación si no lo desea.')}` },

    { id: 'gravacao', title: 'Cámaras: grabación', html: `
      <p>El hub graba el mismo vídeo que muestra en vivo (sin recodificar), en <b>1 archivo MP4 por hora y por cámara</b>, en la carpeta de grabaciones. Los archivos se llaman <code>NOMBRE_AAAAMMDDHH.mp4</code>. Para verlos, use <b>Cámaras → Grabaciones</b> (Ver, Descargar, Eliminar y selección múltiple).</p>
      <h3>Interruptor general</h3>
      <p><b>Grabar las cámaras</b> activa o desactiva todo. Viene desactivado por defecto. Activado, el hub mantiene la conexión con las cámaras las 24 horas del día.</p>
      <h3>Por cámara</h3>
      <table class="help-t"><tr><th>Opción</th><th>Qué hace</th></tr>
        <tr><td>Activar/desactivar</td><td>Graba o no esta cámara (vale si el interruptor general está activado).</td></tr>
        <tr><td>Modo: Continuo</td><td>Graba todo el tiempo.</td></tr>
        <tr><td>Modo: Solo con movimiento</td><td>Graba solo cuando detecta movimiento, con unos segundos antes y 15 segundos después. Una hora sin movimiento no crea archivo.</td></tr>
        <tr><td>Sensibilidad (1 a 5)</td><td>Solo en el modo movimiento. Una más alta detecta movimientos pequeños, pero se dispara con sombras, TV y cambios de luz. Si graba demasiado, bájela.</td></tr>
        <tr><td>Conservar durante</td><td>Las grabaciones más antiguas que esto se borran solas (1 a 365 días; por defecto 7).</td></tr>
        <tr><td>Límite de espacio</td><td>Si se supera este total en GB, el hub borra las más antiguas <b>de esa cámara</b>. “Sin límite” no borra por tamaño.</td></tr></table>
      <p>Debajo del nombre de cada cámara aparece el estado: <i>grabando ahora</i>, <i>esperando movimiento</i>, <i>iniciando…</i> o el mensaje de error, más la cantidad de archivos y el espacio usado.</p>
      <h3>Carpeta de las grabaciones</h3>
      <p>En la tarjeta <b>Grabaciones: carpeta y Google Drive</b>. Por defecto es <code>~/securityCam (carpeta personal de quien ejecuta el hub)</code>. El hub crea la carpeta si no existe. <b>Vale después de reiniciar el hub.</b> Cambiar la carpeta no mueve los archivos antiguos.</p>
      ${tip('la limpieza (retención y límite de GB) se ejecuta cada 10 minutos y solo toca archivos con el nombre de este sistema, nunca otros archivos de la carpeta.')}` },

    { id: 'drive', title: 'Cámaras: envío a Google Drive', html: `
      <p>Cada día el hub copia las grabaciones nuevas a Google Drive usando <b>rclone</b>. Las grabaciones locales siguen la retención de la cámara; la copia en Drive también respeta los mismos días.</p>
      <h3>Antes de activarlo</h3>
      <ol><li><code>rclone</code> debe estar instalado en el servidor y ya vinculado a Google Drive (<code>rclone config</code> crea un “remote”, por ejemplo <code>GDrive</code>).</li>
        <li>Pruebe en el servidor: <code>rclone lsd GDrive:</code> debe listar sus carpetas de Drive.</li></ol>
      <h3>Campos (tarjeta “Grabaciones: carpeta y Google Drive”)</h3>
      <table class="help-t"><tr><th>Campo</th><th>Qué poner</th></tr>
        <tr><td>Copiar las grabaciones a Google Drive todos los días</td><td>Activa el envío automático.</td></tr>
        <tr><td>Destino en rclone</td><td>Con el formato <code>REMOTE:Carpeta</code>, p. ej.: <code>GDrive:CameraSeguranca</code>. Dentro el hub crea una subcarpeta por día y por cámara. Al activarlo se comprueba el formato.</td></tr>
        <tr><td>Hora del envío automático</td><td>Hora en punto en que empieza la ronda (p. ej.: 03:00). Si falla, lo intenta de nuevo hasta 3 veces dentro de una ventana de unas 22 horas.</td></tr>
        <tr><td>Archivo más grande enviado entero (MB)</td><td>Los archivos mayores se dividen en partes, cada una reproducible por sí sola. 0 usa el valor por defecto de 1024 MB.</td></tr></table>
      <p>Todo esto <b>solo vale después de reiniciar el hub</b>.</p>
      <h3>En el día a día (pestaña Cámaras, tarjeta “Google Drive”)</h3>
      <ul><li><b>Sincronizar todo</b>: envía ahora lo que esté pendiente de todas las cámaras.</li>
        <li><b>Sincronizar</b> (dentro de cada cámara): envía solo esa cámara.</li>
        <li>El interruptor <b>Enviar a Google Drive</b> de cada cámara la saca de las rondas automáticas.</li>
        <li>La tarjeta muestra el último mensaje, el número de intentos y la hora de la próxima ronda.</li></ul>` },

    { id: 'agendas', title: 'Agendas (iCal)', html: `
      <p>Las agendas aparecen en la pantalla de inicio (próximo evento, eventos del día, modo nocturno). Cada agenda es un calendario de Google leído por su <b>dirección secreta iCal</b>.</p>
      <h3>Dónde conseguir la dirección</h3>
      <ol><li>En <b>Google Calendar</b> (en el ordenador), haga clic en los tres puntos de la agenda &rarr; <b>Configuración y uso compartido</b>.</li>
        <li>Desplácese hasta <b>Integrar calendario</b>.</li>
        <li>Copie la <b>Dirección secreta en formato iCal</b> (termina en <code>basic.ics</code>).</li></ol>
      ${warn('esa dirección da acceso de lectura a toda la agenda. No la comparta. Si se filtra, genere otra con “Restablecer” en el mismo lugar.')}
      <h3>Añadir</h3>
      <ol><li><b>+ Añadir agenda</b>.</li>
        <li><b>Nombre</b>: aparece en los eventos (p. ej.: Pareja, Fulano, Guardias).</li>
        <li><b>Color</b>: el punto de los eventos de esa agenda. Las agendas antiguas con degradado solo cambian de color si usted elige otro.</li>
        <li><b>Dirección iCal</b>: pegue la dirección secreta.</li>
        <li><b>Guardar agendas</b>. El hub <b>abre la dirección antes de guardarla</b>: si no es un iCal válido, muestra el error y no guarda.</li></ol>
      <h3>Detalles</h3>
      <ul><li>El orden de la lista es el orden de los bloques en la pantalla de inicio. Las agendas sin eventos ese día quedan fuera.</li>
        <li>Quitar borra la agenda del panel (no toca Google Calendar).</li>
        <li>El hub guarda la agenda unos minutos (caché en Integraciones → “Agendas: conservar durante”). Si Google falla, mantiene la última versión buena.</li>
        <li>Máximo de 12 agendas.</li></ul>` },

    { id: 'transito', title: 'Tráfico', html: `
      <p>La tarjeta “Desplazamiento” de la pantalla de inicio muestra el tiempo de cada persona hasta el destino, con el nivel (fluido, moderado, congestionado), la hora de salida y un minimapa. Los datos vienen de la <b>Google Routes API</b>.</p>
      <h3>Paso 1: clave y casa</h3>
      <ul><li><b>Clave de la API Google Routes</b>: cree en Google Cloud Console un proyecto, active la <b>Routes API</b>, active la facturación (hay una cuota gratuita mensual) y genere una clave. Péguela en el campo; queda solo en el servidor. En blanco mantiene la actual.</li>
        <li>Si restringió la clave por <b>dirección IP</b>, active en Integraciones la opción “Usar solo IPv4”, porque Google suele permitir solo la IPv4 pública.</li>
        <li><b>Dirección de casa</b>: origen predeterminado de todas las rutas. Escríbala completa, con número y ciudad (p. ej.: <code>Calle Ejemplo 100, São Paulo, SP</code>). Sin clave o sin dirección de casa el panel muestra datos de <b>ejemplo</b>.</li></ul>
      <h3>Paso 2: personas</h3>
      <p>Cada persona se convierte en una tarjeta de la pantalla de inicio. <b>+ Añadir persona</b> (hasta 8) y <b>Quitar</b> al final de su bloque.</p>
      <ul><li><b>Nombre de la persona</b>: título de la tarjeta.</li>
        <li><b>Necesita llegar a las</b>: con esto la tarjeta calcula “Salga a las HH:MM” (hora de llegada menos el tiempo del trayecto). Desaparece cuando esa hora ya pasó.</li></ul>
      <h3>Cuándo consultar la API de rutas</h3>
      <p>Es la regla que más gasta o ahorra cuota. Elíjala por persona:</p>
      <table class="help-t"><tr><th>Opción</th><th>Cómo funciona</th><th>Úsela cuando</th></tr>
        <tr><td><b>Solo cuando lo pida</b></td><td>La tarjeta muestra el botón <b>Ver tráfico</b>. Al tocarlo, el hub consulta la API y el resultado queda en pantalla 10 minutos. Toques repetidos en menos de 5 minutos reutilizan la consulta.</td><td>Horario variable; quiere ahorrar cuota.</td></tr>
        <tr><td><b>Automático en una ventana de horario</b></td><td>El hub solo consulta dentro de la ventana (p. ej.: 06:00 a 08:00). Antes, la tarjeta dice “Disponible a las 06:00”. Después de la ventana muestra el último resultado, sin nueva consulta. Pasada la hora de <b>“Ocultar la tarjeta después de las”</b> (p. ej.: 09:00) la tarjeta muestra “Solo por la mañana”.</td><td>Rutina fija por la mañana.</td></tr>
        <tr><td><b>Siempre</b></td><td>Consulta en cada actualización del panel, respetando el intervalo de Integraciones (“Tráfico: consultar como máximo cada”, por defecto 10 min).</td><td>Pocas personas y cuota de sobra.</td></tr></table>
      <h3>Paso 3: rutas de la persona</h3>
      <p>Una ruta es un tramo que vale en ciertos días de la semana. <b>+ Añadir ruta</b>:</p>
      <table class="help-t"><tr><th>Campo</th><th>Para qué sirve</th></tr>
        <tr><td>Nombre de la ruta</td><td>Aparece en la tarjeta: “Fulano → Oficina”.</td></tr>
        <tr><td>Origen</td><td>En blanco = la dirección de casa. Rellénelo para tramos que no salen de casa (p. ej.: del trabajo a la escuela).</td></tr>
        <tr><td>Destino</td><td>Dirección completa de llegada.</td></tr>
        <tr><td>A partir de</td><td>Hora (opcional) desde la que esta ruta vale en el día. Sirve para cambiar de tramo durante el día.</td></tr>
        <tr><td>Teletrabajo</td><td>Márquelo en los días sin desplazamiento: la tarjeta muestra “Teletrabajo hoy” y no consulta la API. Los campos de origen y destino desaparecen.</td></tr>
        <tr><td>Días (Lun…Dom)</td><td>Marque los días en que vale la ruta. Hay que marcar al menos uno.</td></tr></table>
      <h3>Ejemplos</h3>
      <ul><li><b>Trabajo de lunes a viernes</b>: una ruta “Oficina”, destino X, días Lun–Vie.</li>
        <li><b>Escuelas y días distintos</b>: ruta “Escuela A” los lunes y jueves, ruta “Escuela B” los martes y viernes, y una ruta “Teletrabajo” (marcando la casilla) el miércoles.</li>
        <li><b>Ida y vuelta</b>: ruta “Ida” (destino trabajo) y ruta “Vuelta” con origen = trabajo, destino = casa y <i>A partir de</i> 17:00. Después de las 17 h vale la “Vuelta”.</li></ul>
      <p>Si una persona no tiene ninguna ruta ese día, la tarjeta muestra “Sin desplazamiento hoy”.</p>
      ${tip('haga clic en <b>Guardar rutas</b> para guardar de una vez la clave, la dirección de casa y todas las personas. Quitar una persona o una ruta guarda al instante.')}` },

    { id: 'painel', title: 'Panel (pantalla de inicio)', html: `
      <p>Ajustes de cómo se comporta la pantalla de inicio. Valen al instante para todos los dispositivos (TV, móvil, navegador): las pantallas abiertas toman el cambio en un máximo de 5 minutos.</p>
      <h3>Ubicación y clima</h3>
      <ul><li><b>Ciudad/Estado</b>: texto del encabezado.</li>
        <li><b>Latitud y Longitud</b>: se usan para buscar el clima (Open-Meteo, sin clave). Use punto como separador (p. ej.: <code>-23.5505</code> y <code>-46.6333</code>). Consejo: en Google Maps, haga clic con el botón derecho en el lugar y copie las coordenadas.</li>
        <li><b>Zona horaria</b>: nombre IANA, p. ej.: <code>America/Sao_Paulo</code>, <code>America/Manaus</code>.</li>
        <li><b>Actualizar el clima cada</b>: de 5 a 240 minutos (por defecto 15).</li></ul>
      <h3>Modo nocturno</h3>
      <p>Desde “Empieza a las” hasta “Termina a las” la pantalla se oscurece y muestra solo el clima y los eventos de mañana (menos brillo, evita dañar la TV). Puede cruzar la medianoche (23:00 hasta 05:00). En la pantalla hay un botón para desactivar el modo nocturno hasta el final de la noche en curso.</p>
      <h3>Cámaras en la pantalla de inicio</h3>
      <p><b>Segundos por cámara</b> (5 a 3600). En el centro de la pantalla de inicio el Clima alterna con cada cámara durante este tiempo (Clima → cámara 1 → cámara 2 → Clima…). Sin cámaras dadas de alta, queda solo el Clima. Para probar rápido, abra <code>index.html?rotate=10</code>.</p>
      <h3>Aviso de caducidad</h3>
      <p><b>Avisar con cuántos días de antelación</b> (0 a 365; por defecto 3). Los artículos que caducan dentro de ese plazo se destacan en ámbar; los caducados, en rojo. El acceso directo “Nevera” del menú también lo usa para avisar de lo que está caducando.</p>
      <h3>Lugares del inventario</h3>
      <p>Los lugares donde se guardan los artículos (Nevera, Congelador, Despensa…). Aparecen como pestañas y como opción al dar de alta un artículo en Nevera y Despensa.</p>
      <ul><li><b>+ Añadir lugar</b>: nombre, color e icono. Guarde con “Guardar”.</li>
        <li>Renombrar no pierde los artículos. El orden es el de la lista.</li>
        <li><b>Quitar</b> solo funciona si el lugar está <b>vacío</b>; si tiene artículos, el hub avisa cuántos son. Mueva o borre los artículos antes.</li>
        <li>Se necesita al menos 1 lugar (máximo 12).</li></ul>
      <h3>Página Vídeos (dirección de reproducción)</h3>
      <p>Los archivos de vídeo los entrega un servidor web de la máquina (nginx). <b>Puerto</b> (por defecto 8080) y <b>Ruta</b> (por defecto <code>/videos/</code>) indican dónde busca el navegador el archivo para reproducirlo o abrirlo en VLC. La carpeta que lista el hub se configura en Integraciones.</p>
      <h3>Festivos locales</h3>
      <p>Complementan los festivos nacionales (que vienen de BrasilAPI). Indique el <b>día y mes</b> (el año se ignora: se repite cada año), el <b>nombre</b> y el <b>tipo</b> (local, estatal, nacional), que cambia el color de la insignia en la pantalla de inicio. Si nunca los editó, valen los de São Paulo (aniversario de la ciudad y 9 de julio). La lista que guarde <b>sustituye</b> a la predeterminada.</p>` },

    { id: 'livros', title: 'Libros (PDF y EPUB)', html: `
      <p>La página <b>Libros</b> guarda PDF y EPUB en el servidor (carpeta <code>~/Livros</code>) y permite subir archivos nuevos desde el propio panel. Cada libro puede tener una <b>categoría</b> y un <b>dueño</b>.</p>
      <table class="help-t"><tr><th>Opción</th><th>Qué hace</th></tr>
        <tr><td>Mostrar la página Libros</td><td>Desactivada, el acceso desaparece del menú y el servidor deja de listar, entregar y recibir libros. Los archivos siguen en la carpeta; al activarla de nuevo, todo vuelve.</td></tr>
        <tr><td>Categorías</td><td>Una por línea. Se convierten en filtros de la página y en opciones al subir o editar un libro. Quitar una categoría de la lista <b>no</b> borra el libro ni la categoría que ya tiene.</td></tr>
        <tr><td>Dueños</td><td>Uno por línea (de quién es el libro). El filtro de dueño aparece en la página cuando algún libro tiene dueño.</td></tr></table>
      <p>Surte efecto al instante; las pantallas abiertas lo toman en un máximo de 5 minutos.</p>
      <h3>En la página Libros</h3>
      <ul><li><b>Subir libro</b>: elija uno o más archivos .pdf o .epub (hasta 300 MB cada uno), la categoría y el dueño. Un nombre repetido recibe “(2)”.</li>
        <li><b>PDF</b> se abre en una pestaña del navegador. <b>EPUB</b> se abre en el lector de la página: las flechas ← → o los botones cambian de capítulo, y el último capítulo leído queda recordado en este dispositivo.</li>
        <li>El lápiz cambia categoría y dueño; la papelera elimina el archivo del servidor (no se puede deshacer).</li></ul>
    ` },
    { id: 'menu', title: 'Menú de accesos directos', html: `
      <p>Son los botones del pie de la pantalla de inicio. Puede añadir, quitar, renombrar y reordenar.</p>
      <h3>Campos de cada acceso directo</h3>
      <table class="help-t"><tr><th>Campo</th><th>Qué es</th></tr>
        <tr><td>Nombre</td><td>Título del botón (hasta 24 letras).</td></tr>
        <tr><td>Descripción</td><td>Texto pequeño debajo (opcional).</td></tr>
        <tr><td>Icono</td><td>Elija de la lista.</td></tr>
        <tr><td>Puerto</td><td><b>Vacío</b>: el acceso abre una página del propio panel/hub. <b>Rellenado</b>: abre otro servicio de la misma máquina en ese puerto (p. ej.: 4533 para Navidrome).</td></tr>
        <tr><td>Ruta</td><td>Empieza por <code>/</code>. Ejemplos: <code>/estoque.html</code>, <code>/camaras.html</code>, <code>/videos.html</code>, <code>/</code>. Puede usar <code>{host}</code> para la dirección de la máquina (p. ej.: <code>/jogos/play/?feed={host}:8080/roms/feed.json</code>).</td></tr></table>
      <h3>Páginas del panel disponibles</h3>
      <p><code>/estoque.html</code> (Nevera), <code>/contatos.html</code> (Servicios), <code>/camaras.html</code> (Cámaras), <code>/videos.html</code> (Vídeos), <code>/livros.html</code> (Libros), <code>/config.html</code> (Config).</p>
      <h3>Reordenar, quitar, restaurar</h3>
      <ul><li>Use las flechas <b>↑</b> y <b>↓</b> y haga clic en <b>Guardar menú</b>.</li>
        <li><b>Quitar</b> guarda al instante.</li>
        <li>El acceso de <b>Config</b> no se puede quitar, para que no pierda el acceso a esta pantalla.</li>
        <li><b>Volver al menú predeterminado</b> descarta sus cambios y vuelve al menú original del panel.</li>
        <li>La primera vez, la lista muestra el menú predeterminado; al guardar, pasa a ser personalizado para todos los dispositivos.</li>
        <li>Los nombres de fábrica (Juegos, Nevera, Cámaras…) se traducen al idioma en uso; un nombre que usted escribió se muestra tal cual.</li></ul>` },

    { id: 'integracoes', title: 'Integraciones', html: `
      <p>Opciones del propio hub. <b>Todas solo valen después de reiniciar el hub</b>: tras guardar, aparece la franja amarilla con el botón <b>Reiniciar el hub ahora</b> (el panel y las cámaras quedan sin servicio unos segundos). Si el botón no aparece, reinicie en el servidor: <code>sudo systemctl restart centraldiogo-hub</code>.</p>
      <h3>Hoja de cálculo de profesionales</h3>
      <p>La página Servicios (profesionales: electricista, fontanero…) puede alimentarse desde una hoja de cálculo de Google.</p>
      <ol><li>En la hoja, deje una pestaña con los encabezados de abajo y <b>publíquela en la web</b> (Archivo &rarr; Compartir &rarr; Publicar en la web) o use el formato <code>https://docs.google.com/spreadsheets/d/ID/gviz/tq?tqx=out:csv&amp;sheet=NOMBRE_DE_LA_PESTAÑA</code>.</li>
        <li>Pegue la dirección en <b>Dirección del CSV</b>. En blanco desactiva la sincronización.</li>
        <li><b>Sincronizar cada</b>: minutos entre lecturas (por defecto 5).</li></ol>
      <p>Encabezados reconocidos (sin distinguir acentos ni mayúsculas): <code>Profissional/Empresa</code> (o Nome), <code>Serviço</code> (o Tipo), <code>Contato</code> (o Telefone), <code>Referência</code> (o Indicação), <code>O que vai fazer</code>, <code>Obs</code>. Los encabezados deben escribirse en portugués, tal como aparecen.</p>
      ${warn('la hoja es la <b>fuente de la verdad</b>: las filas nuevas entran, los cambios actualizan y <b>las filas que salgan de la hoja se borran</b> de la página Servicios.')}
      <h3>Juegos y logros</h3>
      <p>Lista juegos y logros de sus cuentas. <b>Actualizar cada</b>: minutos entre actualizaciones completas (por defecto 360 = 6 horas). Cada servicio:</p>
      <table class="help-t"><tr><th>Servicio</th><th>Qué rellenar</th></tr>
        <tr><td>Steam</td><td><b>Usuario</b>: la URL personalizada del perfil o el SteamID64. <b>Clave de la API</b>: genérela en <code>steamcommunity.com/dev/apikey</code>. El perfil debe tener los detalles de juegos públicos.</td></tr>
        <tr><td>PlayStation</td><td><b>Usuario</b>: solo para mostrar. <b>Cookie npsso</b>: con la sesión iniciada en su cuenta PSN, abra <code>ca.account.sony.com/api/v1/ssocookie</code> y copie el valor de <code>npsso</code>. Vale unos 2 meses: si aparece “npsso inválido o caducado”, genere otro.</td></tr>
        <tr><td>Xbox</td><td><b>Gamertag</b>. Client ID y secret son opcionales (solo si usa una app propia en Azure). Tras guardar y reiniciar, autorice la cuenta una vez: botón <b>Conectar Xbox</b> del panel de juegos, o en el servidor <code>./hub -config hub.config.json -xbox-login</code>.</td></tr>
        <tr><td>RetroAchievements</td><td><b>Usuario</b> y <b>Clave de la API</b> (en <code>retroachievements.org/settings</code>, “Web API Key”).</td></tr></table>
      <p>Los campos de clave quedan en blanco tras guardar y muestran “configurada”. Marque “borrar la configurada” para quitar una clave.</p>
      <h3>Vídeos, caché y red</h3>
      <ul><li><b>Carpeta de vídeos en el servidor</b>: la carpeta que lista la página Vídeos (por defecto <code>~/Videos</code>). En blanco desactiva la lista. La dirección de reproducción está en Panel.</li>
        <li><b>Agendas: conservar durante</b>: minutos que el hub reutiliza la agenda antes de leer Google otra vez (por defecto 5).</li>
        <li><b>Tráfico: consultar como máximo cada</b>: intervalo mínimo entre consultas a la API (por defecto 10). Auméntelo para gastar menos cuota.</li>
        <li><b>Usar solo IPv4</b>: necesario si la clave de Google tiene restricción de IP.</li></ul>` },

    { id: 'idioma', title: 'Idioma', html: `
      <p>La interfaz (pantalla de inicio, Nevera y Despensa, Servicios, Cámaras, Vídeos y la propia Config, incluida esta Ayuda) existe en <b>cuatro idiomas</b>: Português, English, Español y 中文 (chino simplificado).</p>
      <table class="help-t"><tr><th>Opción</th><th>Efecto</th></tr>
        <tr><td>Automático</td><td>Sigue el idioma del navegador del dispositivo. Si no es uno de los cuatro, usa portugués.</td></tr>
        <tr><td>Português, English, Español, 中文</td><td>Fija el idioma elegido en este dispositivo.</td></tr></table>
      <ul><li>La elección vale <b>solo para el dispositivo en el que la hizo</b> (se guarda en su navegador): la TV puede quedarse en portugués y el móvil en inglés. Para la TV, abra Config en la propia TV.</li>
        <li>Al cambiar, la pantalla se recarga. También se puede forzar desde la dirección: <code>index.html?lang=es</code> (<code>pt</code>, <code>en</code>, <code>es</code>, <code>zh</code>); <code>?lang=auto</code> vuelve al automático.</li>
        <li><b>Qué se traduce</b>: los textos de la interfaz, las fechas y los números (con el formato del idioma), los nombres de fábrica del menú (Juegos, Nevera, Cámaras…), de los lugares del inventario (Nevera, Congelador, Despensa) y los festivos nacionales.</li>
        <li><b>Qué no se traduce</b>: lo que usted mismo dio de alta (nombre de cámara, agenda, artículo, profesional, acceso directo renombrado, festivo local) y los eventos de Google Calendar. Los avisos de error que envía el servidor se traducen cuando son conocidos; los demás aparecen tal como los escribió el servidor (en portugués).</li>
        <li>La ciudad del clima, la zona horaria y los festivos vienen de su ubicación, no del idioma: cambiar de idioma no cambia la ciudad.</li></ul>` },

    { id: 'compat', title: 'Modo compatibilidad (TV)', html: `
      <p>Los navegadores de TV (Samsung/Tizen, LG/webOS, Chromecast y similares) normalmente tienen <b>un único decodificador de vídeo</b> y no pueden mostrar varias cámaras en vivo al mismo tiempo. En el modo compatibilidad la página Cámaras muestra <b>una cámara a la vez</b>, con botones con los nombres para cambiar. Las demás quedan desconectadas.</p>
      <table class="help-t"><tr><th>Opción</th><th>Efecto</th></tr>
        <tr><td>Automático</td><td>Se activa solo cuando el navegador parece de TV. El texto de abajo dice si se detectó una TV.</td></tr>
        <tr><td>Siempre activado</td><td>Fuerza una cámara a la vez (útil en una tableta o un PC modesto).</td></tr>
        <tr><td>Siempre desactivado</td><td>Todas las cámaras al mismo tiempo, incluso en TV.</td></tr></table>
      <ul><li>La elección vale <b>solo para el dispositivo en el que la hizo</b> (se guarda en su navegador). Para la TV, abra Config en la propia TV.</li>
        <li>Con una sola cámara dada de alta, nada cambia.</li>
        <li>La pantalla de inicio ya muestra una cámara a la vez en la rotación, así que no se ve afectada.</li></ul>` },

    { id: 'problemas', title: 'Solución de problemas', html: `
      <table class="help-t"><tr><th>Síntoma</th><th>Qué comprobar</th></tr>
        <tr><td>“Sin el hub: abra el panel con la dirección del servidor de la casa”</td><td>Ábralo con la dirección del servidor (p. ej.: <code>http://192.168.0.10/config.html</code>), no desde un archivo ni desde otra dirección.</td></tr>
        <tr><td>La cámara dice “Conectando…” o “cámara no disponible”</td><td>¿IP/puerto/contraseña correctos? ¿La cámara está encendida y en la red? ¿Contraseña con un carácter especial sin codificar? ¿Se equivocó de contraseña varias veces (bloqueo de unos minutos)? Pruebe la misma dirección en VLC.</td></tr>
        <tr><td>“La cámara no entrega H.264”</td><td>Use el substream (<code>subtype=1</code>). El flujo principal suele ser H.265.</td></tr>
        <tr><td>La imagen desaparece solo en la TV</td><td>Active el <b>Modo compatibilidad</b> (pestaña Compatibilidad).</td></tr>
        <tr><td>No aparecen las miniaturas de las cámaras</td><td>Necesitan <code>ffmpeg</code> instalado en el servidor (<code>sudo apt install ffmpeg</code>).</td></tr>
        <tr><td>“no pude leer esta agenda”</td><td>La dirección iCal es incorrecta, se restableció en Google o el servidor no tiene internet. Copie de nuevo la dirección secreta.</td></tr>
        <tr><td>El tráfico muestra “ejemplo”</td><td>Falta la clave de la API o la dirección de casa, o el hub no pudo hablar con Google.</td></tr>
        <tr><td>Tráfico con la etiqueta “caché”</td><td>La última consulta falló y el panel muestra la anterior. Revise la clave, la cuota/facturación de Google y la opción “solo IPv4”.</td></tr>
        <tr><td>La tarjeta de tráfico dice “Disponible a las…” o “Solo por la mañana”</td><td>Es la regla de ventana de la persona. Ajuste la ventana, o cambie a “Siempre”/“Solo cuando lo pida”.</td></tr>
        <tr><td>Cambié una opción de Integraciones y no cambió nada</td><td>Esas opciones solo valen después de reiniciar el hub (franja amarilla en la pantalla).</td></tr>
        <tr><td>Cambié el Panel y la TV no cambió</td><td>Espere hasta 5 minutos o recargue la pantalla de la TV.</td></tr>
        <tr><td>La pantalla se abrió en otro idioma</td><td>En <b>Config → Idioma</b>, elija el idioma del dispositivo. El valor “Automático” sigue el navegador.</td></tr>
        <tr><td>Algunos mensajes de error siguen en portugués</td><td>Son mensajes del servidor que aún no están traducidos. Solo cambian de idioma cuando figuran en la lista conocida del panel.</td></tr>
        <tr><td>“origen no permitido para guardar”</td><td>Abrió Config con una dirección distinta de la del hub (p. ej.: un puerto de desarrollo). Ábrala con la dirección del servidor.</td></tr>
        <tr><td>“Este hub es antiguo y no tiene estas opciones”</td><td>El servidor aún tiene una versión antigua del hub. Haga el despliegue de la versión nueva.</td></tr>
        <tr><td>No puedo quitar un lugar del inventario</td><td>Todavía tiene artículos. Mueva o borre los artículos e inténtelo de nuevo.</td></tr>
        <tr><td>Falla el envío a Drive</td><td>Ejecute en el servidor <code>rclone lsd REMOTE:</code>; si pide iniciar sesión, rehaga <code>rclone config</code>. Vea el mensaje de error en la tarjeta “Google Drive”.</td></tr></table>
      <h3>Volver al original</h3>
      <p>Para deshacer todo lo hecho desde Config, detenga el hub, borre (o renombre) <code>hub.settings.json</code> en el servidor y vuelva a iniciarlo: valen otra vez el <code>hub.config.json</code> y los valores predeterminados del panel.</p>` }
  ]);
})(window.CD = window.CD || {});

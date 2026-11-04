(function (CD) {
  // Config → Help manual in English. Same sections (ids) as help.pt.js; keep the button and tab names identical to the ones in en.js.
  const tip = t => '<div class="help-tip"><b>Tip:</b> ' + t + '</div>';
  const warn = t => '<div class="help-warn"><b>Warning:</b> ' + t + '</div>';

  CD.i18n.addHelp('en', [
    { id: 'comecar', title: 'How Config works', html: `
      <p>Config is where you adjust everything that used to be hidden in files: cameras, calendars, traffic, menu, the weather city, holidays and the integration accounts. It only works <b>when opened through the address of the home server</b> (the hub), because the hub is what stores the settings.</p>
      <h3>The tabs</h3>
      <table class="help-t"><tr><th>Tab</th><th>What it is for</th></tr>
        <tr><td>Server</td><td>CPU and memory usage, and the buttons to free up memory.</td></tr>
        <tr><td>Cameras</td><td>Camera list, recording, the recordings folder and the upload to Google Drive.</td></tr>
        <tr><td>Calendars &amp; routes</td><td>Calendars (iCal) and each person’s traffic.</td></tr>
        <tr><td>Panel</td><td>City and weather, night mode, camera rotation, expiry warning, stock places, videos and local holidays.</td></tr>
        <tr><td>Books</td><td>Turns the Books page on or off and sets the book categories and owners.</td></tr>
        <tr><td>Menu</td><td>The shortcut buttons at the bottom of the home screen.</td></tr>
        <tr><td>Integrations</td><td>Professionals spreadsheet, game accounts, videos folder, cache times and network.</td></tr>
        <tr><td>Language</td><td>Interface language: Portuguese, English, Spanish or Chinese (applies to this device only).</td></tr>
        <tr><td>Compatibility</td><td>Mode for TVs that can only play one video at a time (applies to this device only).</td></tr>
        <tr><td>Help</td><td>This page.</td></tr></table>
      <h3>Saving</h3>
      <ul><li>Each card has its own <b>Save</b> button. Saving one card does not save the others.</li>
        <li>If the hub rejects a value, a message appears <span class="cfg-err">in red</span> on the card itself and <b>what you typed stays there</b> so you can fix it.</li>
        <li><b>Remove</b> buttons take effect immediately (they ask for confirmation). New rows you have not saved yet disappear without contacting the server.</li></ul>
      <h3>When a change takes effect</h3>
      <table class="help-t"><tr><th>What you changed</th><th>When it takes effect</th></tr>
        <tr><td>Cameras, calendars, traffic, menu</td><td>Immediately, on the server. Screens that are already open update the menu and the panel within 5 minutes (or when reloaded).</td></tr>
        <tr><td>Panel tab (city, night mode, rotation, expiry, places, videos, holidays)</td><td>Immediately; open screens pick it up within 5 minutes.</td></tr>
        <tr><td>Recordings folder, Drive, spreadsheet, games, videos folder, cache, IPv4</td><td><b>Only after the hub restarts.</b> The screen shows a yellow “waiting for restart” banner with the <b>Restart the hub now</b> button.</td></tr>
        <tr><td>Language and compatibility mode</td><td>Immediately, on this device only (the screen reloads when you change the language).</td></tr></table>
      <h3>Where it is stored</h3>
      <p>Everything you edit is kept in <code>hub.settings.json</code>, in the same folder as <code>hub.config.json</code> on the server. The original configuration file still applies to everything you have not edited. To <b>take a section back to the original</b>, delete its key in <code>hub.settings.json</code> (or the whole file) and restart the hub. Keep a copy of <code>hub.settings.json</code> as a backup.</p>
      <h3>Secrets (passwords, keys, tokens)</h3>
      <p>Camera passwords, secret calendar addresses and API keys <b>never come back to the screen</b>: they show up masked (<code>*****</code>) or as “set”. Leaving the field <b>blank means keeping the current value</b>. To change it, type the new value. Some fields have a “delete the saved one” option.</p>
      ${warn('the hub is open to your home network and <b>has no login</b>: anyone connected to your Wi-Fi can open Config. Do not expose the hub to the internet.')}` },

    { id: 'servidor', title: 'Server', html: `
      <h3>Meters</h3>
      <p>They show the CPU and memory usage of the hub itself and of the whole machine, refreshed every 5 seconds. If the machine’s memory is almost full, the panel may become slow: use “Free up memory”.</p>
      <h3>Free up memory</h3>
      <ul><li><b>Turn off the desktop</b>: stops the server’s graphical environment (screen, browser, desktop audio). It does not affect the hub, the panel or the cameras. It comes back by itself the next time the machine restarts.</li>
        <li><b>Shut down the hub</b>: stops the hub. The panel, cameras and calendar go offline until someone runs <code>sudo systemctl start centraldiogo-hub</code> on the server.</li></ul>
      <p>Both buttons need <b>two taps</b>: the first arms the button (“Confirm?”, for 5 seconds) and the second runs it. They only work on the Linux server, with the polkit rule installed by <code>install.sh</code>.</p>` },

    { id: 'cameras', title: 'Cameras: setup', html: `
      <p>Under <b>Cameras → Cameras</b> are the cameras that appear on the Cameras page and in the home screen rotation.</p>
      <h3>Adding</h3>
      <ol><li>Click <b>+ Add camera</b>.</li>
        <li><b>Name</b>: how it appears above the picture (e.g. Garage). It also becomes the name of the recording files.</li>
        <li><b>RTSP address</b>: the camera’s video address (see the examples below).</li>
        <li>Click <b>Save cameras</b>. The camera shows up right away, with no restart.</li></ol>
      <h3>RTSP addresses</h3>
      <table class="help-t"><tr><th>Type</th><th>Address</th></tr>
        <tr><td>Intelbras / Dahua (Mibo Smart, iM)</td><td><code>rtsp://admin:PASSWORD@192.168.0.10:554/cam/realmonitor?channel=1&amp;subtype=1</code></td></tr>
        <tr><td>Camera without a password (e.g. “Hankvision” models)</td><td><code>rtsp://192.168.0.11:554/live/mpeg4</code></td></tr></table>
      <ul><li><b>Use the substream (<code>subtype=1</code>)</b>: it is H.264, which the hub relays to the browser. On many cameras <code>subtype=0</code> is H.265 and <b>does not work</b>. The hub reports “the camera does not deliver H.264 at this address”.</li>
        <li>The user is usually <code>admin</code> and the password is the one set in the camera’s app (Mibo Smart). Getting the password wrong several times locks the camera for a few minutes.</li>
        <li><b>Special characters in the password</b> must be encoded: <code>@</code> = <code>%40</code>, <code>!</code> = <code>%21</code>, <code>#</code> = <code>%23</code>, <code>:</code> = <code>%3A</code>, <code>/</code> = <code>%2F</code>.</li>
        <li>Reserve the camera’s IP in the router (fixed DHCP). If the IP changes, the camera stops working until you fix the address here.</li>
        <li>No audio: the live video is always muted.</li></ul>
      <h3>Editing and removing</h3>
      <ul><li>To <b>rename</b>, change the name and save. Note: new files start using the new name.</li>
        <li>To <b>change the password/address</b>, type the whole new address. Blank keeps the current one (the field shows the current one masked).</li>
        <li>The <b>order</b> of the list is the order on the Cameras page and in the rotation.</li>
        <li><b>Remove</b> turns the camera off immediately and closes its recording file. Its old recordings stay in the folder, but with no owner: they follow the default retention (7 days) and are then deleted. Download first whatever you want to keep.</li>
        <li>Maximum of 16 cameras.</li></ul>
      ${tip('a <b>new</b> camera starts recording by itself if the master switch “Record the cameras” is on. Turn it off in the recording card if you do not want that.')}` },

    { id: 'gravacao', title: 'Cameras: recording', html: `
      <p>The hub records the same video it shows live (without re-encoding), as <b>1 MP4 file per hour and per camera</b>, in the recordings folder. Files are named <code>NAME_YYYYMMDDHH.mp4</code>. To watch them, use <b>Cameras → Recordings</b> (Watch, Download, Delete and bulk selection).</p>
      <h3>Master switch</h3>
      <p><b>Record the cameras</b> turns everything on or off. It is off by default. When on, the hub keeps the connection to the cameras 24 hours a day.</p>
      <h3>Per camera</h3>
      <table class="help-t"><tr><th>Option</th><th>What it does</th></tr>
        <tr><td>On/off</td><td>Records or not this camera (applies if the master switch is on).</td></tr>
        <tr><td>Mode: Continuous</td><td>Records all the time.</td></tr>
        <tr><td>Mode: Motion only</td><td>Records only when it detects motion, with a few seconds before and 15 seconds after. An hour with no motion creates no file.</td></tr>
        <tr><td>Sensitivity (1 to 5)</td><td>Motion mode only. Higher picks up small movements, but triggers on shadows, TVs and changes in light. If it records too much, lower it.</td></tr>
        <tr><td>Keep for</td><td>Recordings older than this are deleted automatically (1 to 365 days; default 7).</td></tr>
        <tr><td>Space limit</td><td>Above this total in GB, the hub deletes the oldest recordings <b>of that camera</b>. “No limit” never deletes by size.</td></tr></table>
      <p>Under each camera’s name you see its state: <i>recording now</i>, <i>waiting for motion</i>, <i>starting…</i> or the error message, plus the number of files and the space used.</p>
      <h3>Recordings folder</h3>
      <p>In the <b>Recordings: folder and Google Drive</b> card. The default is <code>~/securityCam (the home folder of whoever runs the hub)</code>. The hub creates the folder if it does not exist. <b>Takes effect after the hub restarts.</b> Changing the folder does not move the old files.</p>
      ${tip('cleanup (retention and GB limit) runs every 10 minutes and only touches files named by this system, never other files in the folder.')}` },

    { id: 'drive', title: 'Cameras: upload to Google Drive', html: `
      <p>Every day the hub copies the new recordings to Google Drive using <b>rclone</b>. Local recordings keep following each camera’s retention; the copy on Drive also respects the same number of days.</p>
      <h3>Before turning it on</h3>
      <ol><li><code>rclone</code> must be installed on the server and already linked to Google Drive (<code>rclone config</code> creates a “remote”, for example <code>GDrive</code>).</li>
        <li>Test it on the server: <code>rclone lsd GDrive:</code> should list your Drive folders.</li></ol>
      <h3>Fields (card “Recordings: folder and Google Drive”)</h3>
      <table class="help-t"><tr><th>Field</th><th>What to enter</th></tr>
        <tr><td>Copy the recordings to Google Drive every day</td><td>Turns on the automatic upload.</td></tr>
        <tr><td>rclone destination</td><td>In the format <code>REMOTE:Folder</code>, e.g. <code>GDrive:CameraSeguranca</code>. Inside it the hub creates one subfolder per day and per camera. The format is checked when you turn it on.</td></tr>
        <tr><td>Automatic upload hour</td><td>The full hour when the run starts (e.g. 03:00). If it fails, it tries again up to 3 times within a window of about 22 hours.</td></tr>
        <tr><td>Largest file uploaded whole (MB)</td><td>Larger files are split into parts, each one playable on its own. 0 uses the default of 1024 MB.</td></tr></table>
      <p>All of this <b>only takes effect after the hub restarts</b>.</p>
      <h3>Day to day (Cameras tab, “Google Drive” card)</h3>
      <ul><li><b>Sync all</b>: uploads now whatever is pending for all cameras.</li>
        <li><b>Sync</b> (inside each camera): uploads that camera only.</li>
        <li>Each camera’s <b>Upload to Google Drive</b> switch takes that camera out of the automatic runs.</li>
        <li>The card shows the last message, the number of attempts and the time of the next run.</li></ul>` },

    { id: 'agendas', title: 'Calendars (iCal)', html: `
      <p>Calendars appear on the home screen (next event, today’s events, night mode). Each calendar is a Google calendar read through its <b>secret iCal address</b>.</p>
      <h3>Where to get the address</h3>
      <ol><li>In <b>Google Calendar</b> (on a computer), click the three dots next to the calendar &rarr; <b>Settings and sharing</b>.</li>
        <li>Scroll down to <b>Integrate calendar</b>.</li>
        <li>Copy the <b>Secret address in iCal format</b> (it ends in <code>basic.ics</code>).</li></ol>
      ${warn('this address gives read access to the whole calendar. Do not share it. If it leaks, generate another one with “Reset” in the same place.')}
      <h3>Adding</h3>
      <ol><li><b>+ Add calendar</b>.</li>
        <li><b>Name</b>: shown on the events (e.g. Couple, Anna, On-call).</li>
        <li><b>Color</b>: the dot on that calendar’s events. Old calendars with a gradient only change color if you pick another one.</li>
        <li><b>iCal address</b>: paste the secret address.</li>
        <li><b>Save calendars</b>. The hub <b>opens the address before storing it</b>: if it is not a valid iCal, it shows the error and does not save.</li></ol>
      <h3>Details</h3>
      <ul><li>The order of the list is the order of the blocks on the home screen. Calendars with no events that day are left out.</li>
        <li>Removing deletes the calendar from the panel (it does not touch Google Calendar).</li>
        <li>The hub keeps the calendar for a few minutes (cache in Integrations → “Calendars: keep for”). If Google fails, it keeps the last good version.</li>
        <li>Maximum of 12 calendars.</li></ul>` },

    { id: 'transito', title: 'Traffic', html: `
      <p>The “Commute” card on the home screen shows each person’s travel time to the destination, with the level (clear, moderate, congested), the time to leave and a mini map. The data comes from the <b>Google Routes API</b>.</p>
      <h3>Step 1: key and home</h3>
      <ul><li><b>Google Routes API key</b>: in the Google Cloud Console, create a project, enable the <b>Routes API</b>, enable billing (there is a free monthly quota) and generate a key. Paste it in the field; it stays only on the server. Blank keeps the current one.</li>
        <li>If you restricted the key by <b>IP address</b>, turn on “Use IPv4 only” in Integrations, because Google usually allows only the public IPv4.</li>
        <li><b>Home address</b>: the default origin of all routes. Write it in full, with number and city (e.g. <code>100 Example St, São Paulo, SP</code>). Without a key or a home address the panel shows <b>sample</b> data.</li></ul>
      <h3>Step 2: people</h3>
      <p>Each person becomes a card on the home screen. <b>+ Add person</b> (up to 8) and <b>Remove</b> at the end of their block.</p>
      <ul><li><b>Person’s name</b>: the card title.</li>
        <li><b>Needs to arrive by</b>: with this, the card calculates “Leave at HH:MM” (arrival time minus the trip time). It disappears once that time has passed.</li></ul>
      <h3>When to query the routes API</h3>
      <p>This is the rule that spends or saves the most quota. Choose it per person:</p>
      <table class="help-t"><tr><th>Option</th><th>How it works</th><th>Use when</th></tr>
        <tr><td><b>Only when I ask</b></td><td>The card shows the <b>Check traffic</b> button. When tapped, the hub queries the API and the result stays on screen for 10 minutes. Repeated taps within 5 minutes reuse the query.</td><td>Variable schedule; you want to save quota.</td></tr>
        <tr><td><b>Automatic within a time window</b></td><td>The hub only queries inside the window (e.g. 06:00 to 08:00). Before it, the card says “Available at 06:00”. After the window it shows the last result, with no new query. Past <b>“Hide the card after”</b> (e.g. 09:00) the card shows “Mornings only”.</td><td>Fixed morning routine.</td></tr>
        <tr><td><b>Always</b></td><td>Queries on every panel refresh, respecting the interval in Integrations (“Traffic: query at most every”, default 10 min).</td><td>Few people and plenty of quota.</td></tr></table>
      <h3>Step 3: the person’s routes</h3>
      <p>A route is a leg that applies on certain days of the week. <b>+ Add route</b>:</p>
      <table class="help-t"><tr><th>Field</th><th>What it is for</th></tr>
        <tr><td>Route name</td><td>Shown on the card: “Anna → Office”.</td></tr>
        <tr><td>Origin</td><td>Blank = the home address. Fill it in for legs that do not start at home (e.g. from work to school).</td></tr>
        <tr><td>Destination</td><td>Full arrival address.</td></tr>
        <tr><td>From</td><td>Time (optional) from which this route applies during the day. Useful to switch legs during the day.</td></tr>
        <tr><td>Working from home</td><td>Tick it on days with no commute: the card shows “Working from home today” and does not query the API. The origin and destination fields disappear.</td></tr>
        <tr><td>Days (Mon…Sun)</td><td>Tick the days the route applies. At least one is required.</td></tr></table>
      <h3>Examples</h3>
      <ul><li><b>Work Monday to Friday</b>: one “Office” route, destination X, days Mon–Fri.</li>
        <li><b>Different schools and days</b>: route “School A” on Mondays and Thursdays, route “School B” on Tuesdays and Fridays, and a “Working from home” route (ticking the box) on Wednesday.</li>
        <li><b>Round trip</b>: route “Outbound” (destination work) and route “Return” with origin = work, destination = home and <i>From</i> 17:00. After 5 pm the “Return” applies.</li></ul>
      <p>If a person has no route that day, the card shows “No commute today”.</p>
      ${tip('click <b>Save routes</b> to store the key, the home address and all the people at once. Removing a person or a route saves immediately.')}` },

    { id: 'painel', title: 'Panel (home screen)', html: `
      <p>Adjustments to how the home screen behaves. They take effect immediately for all devices (TV, phone, browser): open screens pick up the change within 5 minutes.</p>
      <h3>Location and weather</h3>
      <ul><li><b>City/State</b>: header text.</li>
        <li><b>Latitude and Longitude</b>: used to fetch the weather (Open-Meteo, no key). Use a dot as the separator (e.g. <code>-23.5505</code> and <code>-46.6333</code>). Tip: in Google Maps, right-click the place and copy the coordinates.</li>
        <li><b>Time zone</b>: IANA name, e.g. <code>America/Sao_Paulo</code>, <code>America/Manaus</code>.</li>
        <li><b>Refresh the weather every</b>: 5 to 240 minutes (default 15).</li></ul>
      <h3>Night mode</h3>
      <p>From “Starts at” until “Ends at” the screen goes dark, showing only the weather and tomorrow’s events (less brightness, avoids burning the TV). It may cross midnight (23:00 until 05:00). There is a button on the screen to turn night mode off until the end of the current night.</p>
      <h3>Cameras on the home screen</h3>
      <p><b>Seconds per camera</b> (5 to 3600). In the middle of the home screen the Weather alternates with each camera for this long (Weather → camera 1 → camera 2 → Weather…). With no cameras registered, only the Weather stays. To test quickly, open <code>index.html?rotate=10</code>.</p>
      <h3>Expiry warning</h3>
      <p><b>Warn how many days ahead</b> (0 to 365; default 3). Items that expire within this period are highlighted in amber; expired ones in red. The “Fridge” shortcut in the menu also uses this to warn about what is expiring.</p>
      <h3>Stock places</h3>
      <p>The places where items are kept (Fridge, Freezer, Pantry…). They show up as tabs and as an option when registering an item in Fridge &amp; Pantry.</p>
      <ul><li><b>+ Add place</b>: name, color and icon. Save with “Save”.</li>
        <li>Renaming does not lose the items. The order is the order of the list.</li>
        <li><b>Remove</b> only works if the place is <b>empty</b>; if it has items, the hub tells you how many. Move or delete the items first.</li>
        <li>At least 1 place is required (maximum 12).</li></ul>
      <h3>Videos page (playback address)</h3>
      <p>The video files are served by a web server on the machine (nginx). <b>Port</b> (default 8080) and <b>Path</b> (default <code>/videos/</code>) tell the browser where to fetch the file to play it or open it in VLC. The folder the hub lists is set in Integrations.</p>
      <h3>Local holidays</h3>
      <p>They complement the national holidays (which come from BrasilAPI). Enter the <b>day and month</b> (the year is ignored: it repeats every year), the <b>name</b> and the <b>type</b> (local, state, national), which changes the badge color on the home screen. If you have never edited them, the São Paulo ones apply (city anniversary and July 9). The list you save <b>replaces</b> the default one.</p>` },

    { id: 'livros', title: 'Books (PDF and EPUB)', html: `
      <p>The <b>Books</b> page keeps PDFs and EPUBs on the server (folder <code>~/Livros</code>) and lets you upload new files from the panel itself. Each book can have a <b>category</b> and an <b>owner</b>.</p>
      <table class="help-t"><tr><th>Option</th><th>What it does</th></tr>
        <tr><td>Show the Books page</td><td>When off, the shortcut disappears from the menu and the server stops listing, delivering and receiving books. The files stay in the folder; turn it on again and everything is back.</td></tr>
        <tr><td>Categories</td><td>One per line. They become filters on the page and options when uploading or editing a book. Removing a category from the list does <b>not</b> delete the book or the category it already has.</td></tr>
        <tr><td>Owners</td><td>One per line (whose book it is). The owner filter shows on the page once some book has an owner.</td></tr></table>
      <p>Takes effect immediately; open screens pick it up within 5 minutes.</p>
      <h3>On the Books page</h3>
      <ul><li><b>Upload book</b>: choose one or more .pdf or .epub files (up to 300 MB each), the category and the owner. A repeated name gets “(2)”.</li>
        <li><b>PDF</b> opens in a browser tab. <b>EPUB</b> opens in the page reader: the ← → arrows or the buttons change chapter, and the last chapter read is remembered on this device.</li>
        <li>The pencil changes category and owner; the trash can deletes the file from the server (it cannot be undone).</li></ul>
    ` },
    { id: 'menu', title: 'Shortcut menu', html: `
      <p>These are the buttons at the bottom of the home screen. You can add, remove, rename and reorder them.</p>
      <h3>Fields of each shortcut</h3>
      <table class="help-t"><tr><th>Field</th><th>What it is</th></tr>
        <tr><td>Name</td><td>Button title (up to 24 letters).</td></tr>
        <tr><td>Description</td><td>Small text below (optional).</td></tr>
        <tr><td>Icon</td><td>Pick from the list.</td></tr>
        <tr><td>Port</td><td><b>Blank</b>: the shortcut opens a page of the panel/hub itself. <b>Filled in</b>: opens another service on the same machine on that port (e.g. 4533 for Navidrome).</td></tr>
        <tr><td>Path</td><td>Starts with <code>/</code>. Examples: <code>/estoque.html</code>, <code>/camaras.html</code>, <code>/videos.html</code>, <code>/</code>. You can use <code>{host}</code> for the machine’s address (e.g. <code>/jogos/play/?feed={host}:8080/roms/feed.json</code>).</td></tr></table>
      <h3>Available panel pages</h3>
      <p><code>/estoque.html</code> (Fridge), <code>/contatos.html</code> (Services), <code>/camaras.html</code> (Cameras), <code>/videos.html</code> (Videos), <code>/livros.html</code> (Books), <code>/config.html</code> (Config).</p>
      <h3>Reorder, remove, restore</h3>
      <ul><li>Use the <b>↑</b> and <b>↓</b> arrows and click <b>Save menu</b>.</li>
        <li><b>Remove</b> saves immediately.</li>
        <li>The <b>Config</b> shortcut cannot be removed, so you never lose access to this screen.</li>
        <li><b>Back to the default menu</b> discards your changes and returns to the panel’s original menu.</li>
        <li>The first time, the list shows the default menu; once you save, it becomes custom for all devices.</li>
        <li>The default names (Games, Fridge, Cameras…) are translated to the language in use; a name you typed is shown exactly as written.</li></ul>` },

    { id: 'integracoes', title: 'Integrations', html: `
      <p>Options of the hub itself. <b>All of them only take effect after the hub restarts</b>: after saving, the yellow banner appears with the <b>Restart the hub now</b> button (the panel and cameras go offline for a few seconds). If the button does not appear, restart on the server: <code>sudo systemctl restart centraldiogo-hub</code>.</p>
      <h3>Professionals spreadsheet</h3>
      <p>The Services page (professionals: electrician, plumber…) can be fed from a Google spreadsheet.</p>
      <ol><li>In the spreadsheet, keep a sheet with the headers below and <b>publish it to the web</b> (File &rarr; Share &rarr; Publish to web) or use the format <code>https://docs.google.com/spreadsheets/d/ID/gviz/tq?tqx=out:csv&amp;sheet=SHEET_NAME</code>.</li>
        <li>Paste the address in <b>CSV address</b>. Blank turns syncing off.</li>
        <li><b>Sync every</b>: minutes between reads (default 5).</li></ol>
      <p>Recognized headers (ignoring accents and capitalization): <code>Profissional/Empresa</code> (or Nome), <code>Serviço</code> (or Tipo), <code>Contato</code> (or Telefone), <code>Referência</code> (or Indicação), <code>O que vai fazer</code>, <code>Obs</code>. The headers must be written in Portuguese, as listed.</p>
      ${warn('the spreadsheet is the <b>source of truth</b>: new rows come in, changes update, and <b>rows that leave the spreadsheet are deleted</b> from the Services page.')}
      <h3>Games and achievements</h3>
      <p>Lists games and achievements from your accounts. <b>Refresh every</b>: minutes between full updates (default 360 = 6 hours). Each service:</p>
      <table class="help-t"><tr><th>Service</th><th>What to fill in</th></tr>
        <tr><td>Steam</td><td><b>User</b>: the profile’s custom URL or the SteamID64. <b>API key</b>: generate it at <code>steamcommunity.com/dev/apikey</code>. The profile must have game details set to public.</td></tr>
        <tr><td>PlayStation</td><td><b>User</b>: display only. <b>npsso cookie</b>: while logged in to your PSN account, open <code>ca.account.sony.com/api/v1/ssocookie</code> and copy the <code>npsso</code> value. It lasts about 2 months: if “npsso invalid or expired” appears, generate another.</td></tr>
        <tr><td>Xbox</td><td><b>Gamertag</b>. Client ID and secret are optional (only if you use your own app in Azure). After saving and restarting, authorize the account once: the <b>Connect Xbox</b> button in the games panel, or on the server <code>./hub -config hub.config.json -xbox-login</code>.</td></tr>
        <tr><td>RetroAchievements</td><td><b>User</b> and <b>API key</b> (at <code>retroachievements.org/settings</code>, “Web API Key”).</td></tr></table>
      <p>Key fields stay blank after saving and show “set”. Tick “delete the saved one” to remove a key.</p>
      <h3>Videos, cache and network</h3>
      <ul><li><b>Videos folder on the server</b>: the folder the Videos page lists (default <code>~/Videos</code>). Blank turns the list off. The playback address is in Panel.</li>
        <li><b>Calendars: keep for</b>: minutes the hub reuses a calendar before reading Google again (default 5).</li>
        <li><b>Traffic: query at most every</b>: minimum interval between API queries (default 10). Increase it to spend less quota.</li>
        <li><b>Use IPv4 only</b>: needed if the Google key has an IP restriction.</li></ul>` },

    { id: 'idioma', title: 'Language', html: `
      <p>The interface (home screen, Fridge &amp; Pantry, Services, Cameras, Videos and Config itself, including this Help) is available in <b>four languages</b>: Português, English, Español and 中文 (Simplified Chinese).</p>
      <table class="help-t"><tr><th>Option</th><th>Effect</th></tr>
        <tr><td>Automatic</td><td>Follows the device’s browser language. If it is not one of the four, Portuguese is used.</td></tr>
        <tr><td>Português, English, Español, 中文</td><td>Fixes the chosen language on this device.</td></tr></table>
      <ul><li>The choice applies <b>only to the device where you made it</b> (it is stored in its browser): the TV can stay in Portuguese and the phone in English. For the TV, open Config on the TV itself.</li>
        <li>The screen reloads when you change it. You can also force it in the address: <code>index.html?lang=en</code> (<code>pt</code>, <code>en</code>, <code>es</code>, <code>zh</code>); <code>?lang=auto</code> goes back to automatic.</li>
        <li><b>What is translated</b>: interface texts, dates and numbers (in the language’s format), the factory names of the menu (Games, Fridge, Cameras…), of the stock places (Fridge, Freezer, Pantry) and the national holidays.</li>
        <li><b>What is not translated</b>: whatever you registered yourself (camera, calendar, item, professional, renamed shortcut or local holiday names) and your Google Calendar events. Error messages sent by the server are translated when they are known; the others appear as the server wrote them (in Portuguese).</li>
        <li>The weather city, the time zone and the holidays come from your location, not from the language: changing the language does not change the city.</li></ul>` },

    { id: 'compat', title: 'Compatibility mode (TVs)', html: `
      <p>TV browsers (Samsung/Tizen, LG/webOS, Chromecast and similar) usually have <b>a single video decoder</b> and cannot show several live cameras at the same time. In compatibility mode the Cameras page shows <b>one camera at a time</b>, with buttons carrying their names to switch. The others stay disconnected.</p>
      <table class="help-t"><tr><th>Option</th><th>Effect</th></tr>
        <tr><td>Automatic</td><td>Turns on by itself when the browser looks like a TV. The text below says whether a TV was detected.</td></tr>
        <tr><td>Always on</td><td>Forces one camera at a time (useful on a tablet or weak PC).</td></tr>
        <tr><td>Always off</td><td>All cameras at the same time, even on a TV.</td></tr></table>
      <ul><li>The choice applies <b>only to the device where you made it</b> (it is stored in its browser). For the TV, open Config on the TV itself.</li>
        <li>With a single camera registered, nothing changes.</li>
        <li>The home screen already shows one camera at a time in the rotation, so it is not affected.</li></ul>` },

    { id: 'problemas', title: 'Troubleshooting', html: `
      <table class="help-t"><tr><th>Symptom</th><th>What to check</th></tr>
        <tr><td>“No hub: open the panel using the address of the home server”</td><td>Open it through the server’s address (e.g. <code>http://192.168.0.10/config.html</code>), not through a file or another address.</td></tr>
        <tr><td>Camera says “Connecting…” or “camera unavailable”</td><td>Are the IP/port/password correct? Is the camera on and on the network? Password with an unencoded special character? Wrong password several times (a few minutes’ lockout)? Test the same address in VLC.</td></tr>
        <tr><td>“The camera does not deliver H.264”</td><td>Use the substream (<code>subtype=1</code>). The main stream is usually H.265.</td></tr>
        <tr><td>Picture disappears only on the TV</td><td>Turn on <b>Compatibility mode</b> (Compatibility tab).</td></tr>
        <tr><td>Camera thumbnails do not appear</td><td>They need <code>ffmpeg</code> installed on the server (<code>sudo apt install ffmpeg</code>).</td></tr>
        <tr><td>“could not read this calendar”</td><td>The iCal address is wrong, was reset in Google, or the server has no internet. Copy the secret address again.</td></tr>
        <tr><td>Traffic shows “sample”</td><td>The API key or the home address is missing, or the hub could not reach Google.</td></tr>
        <tr><td>Traffic with a “cached” tag</td><td>The last query failed and the panel shows the previous one. Check the key, Google quota/billing and the “IPv4 only” option.</td></tr>
        <tr><td>The traffic card says “Available at…” or “Mornings only”</td><td>It is the person’s window rule. Adjust the window, or switch to “Always”/“Only when I ask”.</td></tr>
        <tr><td>I changed an Integrations option and nothing changed</td><td>Those options only take effect after the hub restarts (yellow banner on the screen).</td></tr>
        <tr><td>I changed the Panel and the TV did not change</td><td>Wait up to 5 minutes or reload the TV screen.</td></tr>
        <tr><td>The screen opened in another language</td><td>In <b>Config → Language</b>, choose the device’s language. The default “Automatic” follows the browser.</td></tr>
        <tr><td>Some error messages are still in Portuguese</td><td>They are server messages that are not translated yet. They only change language when they are in the panel’s known list.</td></tr>
        <tr><td>“origin not allowed to save”</td><td>You opened Config through an address different from the hub’s (e.g. a development port). Open it through the server’s address.</td></tr>
        <tr><td>“This hub is out of date and does not have these options”</td><td>The server is still running an old version of the hub. Deploy the new version.</td></tr>
        <tr><td>I cannot remove a stock place</td><td>There are still items in it. Move or delete the items and try again.</td></tr>
        <tr><td>Upload to Drive fails</td><td>Run <code>rclone lsd REMOTE:</code> on the server; if it asks for a login, redo <code>rclone config</code>. See the error message in the “Google Drive” card.</td></tr></table>
      <h3>Back to the original</h3>
      <p>To undo everything done through Config, stop the hub, delete (or rename) <code>hub.settings.json</code> on the server and start it again: <code>hub.config.json</code> and the panel defaults apply again.</p>` }
  ]);
})(window.CD = window.CD || {});

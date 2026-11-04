(function (CD) {
  const SUN = '<circle cx="12" cy="12" r="4.2"/><path d="M12 2.5v2.4M12 19.1v2.4M4.6 4.6l1.7 1.7M17.7 17.7l1.7 1.7M2.5 12h2.4M19.1 12h2.4M4.6 19.4l1.7-1.7M17.7 6.3l1.7-1.7"/>';
  const CLOUD = '<path d="M7 18.5h10.5a3.8 3.8 0 0 0 .6-7.55 5 5 0 0 0-9.6-1.9A4.2 4.2 0 0 0 7 18.5Z"/>';
  const CLOUD_UP = '<path d="M7 15.5h10.5a3.8 3.8 0 0 0 .6-7.55 5 5 0 0 0-9.6-1.9A4.2 4.2 0 0 0 7 15.5Z"/>';
  const PARTLY = '<circle cx="8.5" cy="8" r="3.4"/><path d="M8.5 2.8v1.6M3.6 8h1.6M13 9.8l-1.3 1.1"/><path d="M9.5 19h8a3.4 3.4 0 0 0 .5-6.76 4.4 4.4 0 0 0-8.34-1.5A3.7 3.7 0 0 0 9.5 19Z"/>';
  const PATHS = {
    sun: SUN,
    partly: PARTLY,
    cloud: CLOUD,
    rain: CLOUD_UP + '<path d="M8.5 18.5l-1 2.2M12.5 18.5l-1 2.2M16.5 18.5l-1 2.2"/>',
    storm: CLOUD_UP + '<path d="M12.5 16.5l-2 3h3l-1.5 3"/>',
    fog: CLOUD_UP + '<path d="M6 18.5h12M8 21.5h8"/>',
    moon: '<path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5Z"/>',
    gamepad: '<rect x="2.5" y="7" width="19" height="10" rx="5"/><path d="M7.5 10v4M5.5 12h4"/><circle cx="15.5" cy="11" r=".8"/><circle cx="18" cy="13" r=".8"/>',
    music: '<path d="M9 18V6l11-2v12"/><circle cx="6.5" cy="18" r="2.5"/><circle cx="17.5" cy="16" r="2.5"/>',
    film: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M7 4v16M17 4v16M3 9h4M3 15h4M17 9h4M17 15h4"/>',
    folder: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/>',
    umbrella: '<path d="M12 3a9 9 0 0 0-9 9h18a9 9 0 0 0-9-9ZM12 12v6.2a2 2 0 0 0 4 0"/>',
    trophy: '<path d="M7 4h10v5a5 5 0 0 1-10 0Z"/><path d="M7 6H4.5a2.5 2.5 0 0 0 2.8 3.4M17 6h2.5a2.5 2.5 0 0 1-2.8 3.4M12 14v3M8.5 20.5h7M9.5 17h5v3.5h-5Z"/>',
    clock: '<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 2"/>',
    refresh: '<path d="M20 11a8 8 0 0 0-14.3-4.3L4 8.5M4 4v4.5h4.5M4 13a8 8 0 0 0 14.3 4.3l1.7-1.8M20 20v-4.5h-4.5"/>',
    back: '<path d="M15 5l-7 7 7 7"/>',
    search: '<circle cx="11" cy="11" r="6.5"/><path d="M20 20l-4.3-4.3"/>',
    home: '<path d="M3 11.5l9-8 9 8"/><path d="M5.5 10v10h4.5v-6h4v6h4.5V10"/>',
    fridge: '<rect x="5" y="2.5" width="14" height="19" rx="2.5"/><path d="M5 10.5h14M8.5 6v1.8M8.5 13.5V17"/>',
    phone: '<path d="M5 4h4l2 5-2.5 1.5a11 11 0 0 0 5 5L15 13l5 2v4a2 2 0 0 1-2 2A16 16 0 0 1 3 6a2 2 0 0 1 2-2Z"/>',
    whatsapp: '<path d="M3.5 20.5l1.3-4.2A8.5 8.5 0 1 1 8 19.3l-4.5 1.2Z"/><path d="M9.2 8.6c-.5 2.4 2.6 5.8 5.6 6.2l1.1-1.2-1.9-1-.8.8a4.4 4.4 0 0 1-1.7-1.7l.8-.8-1-1.9-1.1 1.6Z"/>',
    desktop: '<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/>',
    book: '<path d="M5 4.5A1.5 1.5 0 0 1 6.5 3H19v15H6.5A1.5 1.5 0 0 0 5 19.5v-15Z"/><path d="M5 19.5A1.5 1.5 0 0 0 6.5 21H19v-3M9 7.5h6"/>',
    mail: '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3.5 7l8.5 6 8.5-6"/>',
    link: '<path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1"/><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1"/>',
    user: '<circle cx="12" cy="8" r="3.5"/><path d="M5 20a7 7 0 0 1 14 0"/>',
    plus: '<path d="M12 5v14M5 12h14"/>',
    edit: '<path d="M4 20h4L19 9a2.8 2.8 0 0 0-4-4L4 16v4ZM13.5 6.5l4 4"/>',
    trash: '<path d="M4 7h16M9.5 7V4.5h5V7M6.5 7l1 13h9l1-13M10 11v6M14 11v6"/>',
    pin: '<path d="M12 21s7-6.2 7-11.5a7 7 0 0 0-14 0C5 14.8 12 21 12 21Z"/><circle cx="12" cy="9.5" r="2.5"/>',
    snow: '<path d="M12 3v18M4.2 7.5l15.6 9M19.8 7.5l-15.6 9M9.5 4.7L12 7.2l2.5-2.5M9.5 19.3L12 16.8l2.5 2.5"/>',
    box: '<path d="M3.5 8L12 3.5 20.5 8v8.5L12 21l-8.5-4.5V8Z"/><path d="M3.5 8L12 12.5 20.5 8M12 12.5V21"/>',
    x: '<path d="M6 6l12 12M18 6L6 18"/>',
    alert: '<path d="M12 3.5l9.5 16.5h-19L12 3.5Z"/><path d="M12 10v4.5M12 17.4v.1"/>',
    check: '<path d="M5 12.5l4.5 4.5L19 7.5"/>',
    camera: '<path d="M3 8a2 2 0 0 1 2-2h2l1.5-2h7L17 6h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/><circle cx="12" cy="12.5" r="3.5"/>',
    gear: '<circle cx="12" cy="12" r="3"/><path d="M12 2.5l1.6 2.6 3-.6.9 2.9 2.9.9-.6 3L22 12l-2.6 1.6.6 3-2.9.9-.9 2.9-3-.6L12 21.5l-1.6-2.6-3 .6-.9-2.9-2.9-.9.6-3L2 12l2.6-1.6-.6-3 2.9-.9.9-2.9 3 .6L12 2.5Z"/>',
    expand:'<path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5"/>'
  };

  // Código WMO (Open-Meteo) -> nome do ícone
  function nameFor(code, isDay) {
    if (code === 0) return isDay === false ? 'moon' : 'sun';
    if (code === 1 || code === 2) return 'partly';
    if (code === 3) return 'cloud';
    if (code === 45 || code === 48) return 'fog';
    if ((code >= 51 && code <= 67) || (code >= 80 && code <= 82)) return 'rain';
    if (code >= 95) return 'storm';
    return 'cloud';
  }

  CD.icons = {
    nameFor,
    svg(name, color, size, stroke) {
      return '<svg viewBox="0 0 24 24" fill="none" stroke="' + (color || 'currentColor') + '" stroke-width="' + (stroke || 1.8) +
        '" stroke-linecap="round" stroke-linejoin="round" style="width:' + size + 'px;height:' + size + 'px;flex-shrink:0">' + PATHS[name] + '</svg>';
    },
    weather(code, isDay, color, size, stroke) { return CD.icons.svg(nameFor(code, isDay), color, size, stroke); }
  };
})(window.CD = window.CD || {});

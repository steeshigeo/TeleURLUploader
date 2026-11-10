module.exports = async (req, res) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

  if (req.method === 'OPTIONS') {
    return res.status(200).end();
  }

  try {
    const protocol = req.headers['x-forwarded-proto'] || 'https';
    const host = req.headers.host || 'jingapps-premium.vercel.app';
    const baseUrl = `${protocol}://${host}`;
    const dataUrl = `${baseUrl}/data/apps.json`;

    let apps = [];
    try {
      const response = await fetch(dataUrl);
      if (response.ok) {
        apps = await response.json();
      }
    } catch (e) {
      apps = [];
    }

    apps = apps.map(app => ({
      ...app,
      thumbnail_url: app.thumbnail ? `${baseUrl}${app.thumbnail}` : null
    }));

    const { id, search, category, limit } = req.query || {};

    if (id) {
      const singleApp = apps.find(a => (a.id || '').toLowerCase() === id.toLowerCase());
      if (!singleApp) {
        return res.status(404).json({
          success: false,
          message: `Application with ID '${id}' not found.`
        });
      }
      return res.status(200).json({
        success: true,
        data: singleApp
      });
    }

    if (category) {
      apps = apps.filter(a => (a.category || '').toLowerCase() === category.toLowerCase());
    }

    if (search) {
      const q = search.toLowerCase();
      apps = apps.filter(a =>
        (a.app_name || '').toLowerCase().includes(q) ||
        (a.modified || '').toLowerCase().includes(q) ||
        (a.note || '').toLowerCase().includes(q)
      );
    }

    if (limit && !isNaN(parseInt(limit))) {
      apps = apps.slice(0, parseInt(limit));
    }

    return res.status(200).json({
      success: true,
      count: apps.length,
      data: apps
    });

  } catch (error) {
    return res.status(500).json({
      success: false,
      message: 'Internal API Error',
      error: error.message
    });
  }
};

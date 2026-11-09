// Static require memaksa Vercel bundler memasukkan public/data/apps.json ke dalam Serverless Package
let catalogData = [];
try {
  catalogData = require('../../public/data/apps.json');
} catch (e) {
  catalogData = [];
}

module.exports = (req, res) => {
  // Set Headers CORS
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

  if (req.method === 'OPTIONS') {
    return res.status(200).end();
  }

  try {
    const protocol = req.headers['x-forwarded-proto'] || 'https';
    const host = req.headers.host || '';
    const baseUrl = host ? `${protocol}://${host}` : '';

    // Salin data katalog agar aman saat dibaca
    let apps = JSON.parse(JSON.stringify(catalogData));

    apps = apps.map(app => ({
      ...app,
      thumbnail_url: app.thumbnail ? `${baseUrl}${app.thumbnail}` : null
    }));

    const { id, search, category, limit } = req.query || {};

    // 1. Filter spesifik Bundle ID
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

    // 2. Filter Kategori
    if (category) {
      apps = apps.filter(a => (a.category || '').toLowerCase() === category.toLowerCase());
    }

    // 3. Filter Query Pencarian
    if (search) {
      const q = search.toLowerCase();
      apps = apps.filter(a =>
        (a.app_name || '').toLowerCase().includes(q) ||
        (a.modified || '').toLowerCase().includes(q) ||
        (a.note || '').toLowerCase().includes(q)
      );
    }

    // 4. Limit Jumlah Data
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

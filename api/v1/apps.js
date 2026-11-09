const fs = require('fs');
const path = require('path');

module.exports = async (req, res) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

  if (req.method === 'OPTIONS') {
    return res.status(200).end();
  }

  try {
    let apps = [];
    const protocol = req.headers['x-forwarded-proto'] || 'https';
    const host = req.headers.host;
    const baseUrl = `${protocol}://${host}`;

    // 1. Coba baca file lokal fs
    const possiblePaths = [
      path.join(process.cwd(), 'public', 'data', 'apps.json'),
      path.join(__dirname, '../../public/data/apps.json'),
      path.join(__dirname, '../public/data/apps.json')
    ];

    let fileFound = false;
    for (const p of possiblePaths) {
      if (fs.existsSync(p)) {
        const rawData = fs.readFileSync(p, 'utf8');
        apps = JSON.parse(rawData || '[]');
        fileFound = true;
        break;
      }
    }

    // 2. Fallback: Jika fs di-isolate Vercel, fetch data via HTTP secara internal
    if (!fileFound && host) {
      try {
        const fetchRes = await fetch(`${baseUrl}/data/apps.json`);
        if (fetchRes.ok) {
          apps = await fetchRes.json();
        }
      } catch (e) {
        // Fallback fetch error ignore
      }
    }

    // Format thumbnail_url lengkap
    apps = apps.map(app => ({
      ...app,
      thumbnail_url: app.thumbnail ? `${baseUrl}${app.thumbnail}` : null
    }));

    const { id, search, category, limit } = req.query;

    if (id) {
      const app = apps.find(a => (a.id || '').toLowerCase() === id.toLowerCase());
      if (!app) {
        return res.status(404).json({
          success: false,
          message: `Application with ID '${id}' not found.`
        });
      }
      return res.status(200).json({
        success: true,
        data: app
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
      message: 'Failed to retrieve API data',
      error: error.message
    });
  }
};

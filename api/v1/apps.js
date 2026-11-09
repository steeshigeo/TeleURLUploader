const https = require('https');
const http = require('http');

module.exports = (req, res) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

  if (req.method === 'OPTIONS') {
    return res.status(200).end();
  }

  const host = req.headers.host || 'tele-url-uploader.vercel.app';
  const protocol = req.headers['x-forwarded-proto'] || 'https';
  const dataUrl = `${protocol}://${host}/data/apps.json`;

  const client = protocol === 'https' ? https : http;

  client.get(dataUrl, (apiRes) => {
    let body = '';
    apiRes.on('data', (chunk) => { body += chunk; });
    
    apiRes.on('end', () => {
      try {
        let apps = JSON.parse(body || '[]');
        const baseUrl = `${protocol}://${host}`;

        apps = apps.map(app => ({
          ...app,
          thumbnail_url: app.thumbnail ? `${baseUrl}${app.thumbnail}` : null
        }));

        const { id, search, category, limit } = req.query || {};

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
      } catch (err) {
        return res.status(200).json({
          success: true,
          count: 0,
          data: [],
          message: "Data catalog is empty or invalid"
        });
      }
    });
  }).on('error', (err) => {
    return res.status(500).json({
      succescat << 'EOF' > api/v1/apps.js
const https = require('https');
const http = require('http');

module.exports = (req, res) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

  if (req.method === 'OPTIONS') {
    return res.status(200).end();
  }

  const host = req.headers.host || 'tele-url-uploader.vercel.app';
  const protocol = req.headers['x-forwarded-proto'] || 'https';
  const dataUrl = `${protocol}://${host}/data/apps.json`;

  const client = protocol === 'https' ? https : http;

  client.get(dataUrl, (apiRes) => {
    let body = '';
    apiRes.on('data', (chunk) => { body += chunk; });
    
    apiRes.on('end', () => {
      try {
        let apps = JSON.parse(body || '[]');
        const baseUrl = `${protocol}://${host}`;

        apps = apps.map(app => ({
          ...app,
          thumbnail_url: app.thumbnail ? `${baseUrl}${app.thumbnail}` : null
        }));

        const { id, search, category, limit } = req.query || {};

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
      } catch (err) {
        return res.status(200).json({
          success: true,
          count: 0,
          data: [],
          message: "Data catalog is empty or invalid"
        });
      }
    });
  }).on('error', (err) => {
    return res.status(500).json({
      success: false,
      message: 'Failed to fetch catalog data',
      error: err.message
    });
  });
};

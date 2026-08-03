const express = require('express');
const http = require('http');
const path = require('path');

const app = express();
const PORT = process.env.PORT || 3000;

// TurboPi configuration
const TURBOPI_IP = process.env.TURBOPI_IP;
const CAMERA_STREAM_URL = process.env.CAMERA_STREAM_URL;
const RPC_URL = process.env.RPC_URL;

if (!TURBOPI_IP || !CAMERA_STREAM_URL || !RPC_URL) {
  console.error('Error: Missing required environment variables.');
  if (!TURBOPI_IP) console.error('  - TURBOPI_IP is not set');
  if (!CAMERA_STREAM_URL) console.error('  - CAMERA_STREAM_URL is not set');
  if (!RPC_URL) console.error('  - RPC_URL is not set');
  process.exit(1);
}

app.use(express.json());
app.use(express.static(path.join(__dirname, 'public')));

// Proxy endpoint for TurboPi Camera MJPEG stream
app.get('/api/stream', (req, res) => {
  console.log(`Proxying stream from ${CAMERA_STREAM_URL}`);
  
  const proxyReq = http.get(CAMERA_STREAM_URL, (proxyRes) => {
    // Forward the headers (like Content-Type: multipart/x-mixed-replace; boundary=...)
    res.writeHead(proxyRes.statusCode, proxyRes.headers);
    proxyRes.pipe(res);
  });

  proxyReq.on('error', (err) => {
    console.error('Error proxying camera stream:', err.message);
    res.status(502).json({ error: 'TurboPi camera stream is offline', details: err.message });
  });

  // If the client closes the connection, abort the proxy request
  req.on('close', () => {
    proxyReq.destroy();
  });
});

// Proxy endpoint for TurboPi JSON-RPC Control
app.post('/api/rpc', async (req, res) => {
  const { method, params } = req.body;

  if (!method) {
    return res.status(400).json({ error: 'Method is required' });
  }

  const payload = {
    jsonrpc: '2.0',
    method: method,
    params: params || [],
    id: 1
  };

  const payloadString = JSON.stringify(payload);

  const options = {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Content-Length': Buffer.byteLength(payloadString)
    },
    timeout: 3000 // 3 seconds timeout
  };

  const rpcReq = http.request(RPC_URL, options, (rpcRes) => {
    let data = '';
    rpcRes.on('data', (chunk) => {
      data += chunk;
    });

    rpcRes.on('end', () => {
      try {
        const jsonResponse = JSON.parse(data);
        res.json(jsonResponse);
      } catch (e) {
        res.status(502).json({ error: 'Invalid JSON response from TurboPi RPC server', details: data });
      }
    });
  });

  rpcReq.on('error', (err) => {
    console.error('Error sending RPC to TurboPi:', err.message);
    res.status(502).json({ error: 'TurboPi RPC server is offline', details: err.message });
  });

  rpcReq.on('timeout', () => {
    rpcReq.destroy();
    res.status(504).json({ error: 'TurboPi RPC request timed out' });
  });

  rpcReq.write(payloadString);
  rpcReq.end();
});

// Fallback to serving index.html for SPA-like experience if needed
app.get('*', (req, res) => {
  res.sendFile(path.join(__dirname, 'public', 'index.html'));
});

app.listen(PORT, () => {
  console.log(`==================================================`);
  console.log(` TurboPi Controller Webapp listening on port ${PORT}`);
  console.log(` Local URL: http://localhost:${PORT}`);
  console.log(`==================================================`);
});

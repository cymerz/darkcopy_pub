import { IncomingMessage, ServerResponse } from 'http';
import http from 'http';

export const config = {
  api: {
    // Disable Next.js body parser so that files up to 100 MB+ can stream
    // directly to the backend without being loaded into Next.js memory or hitting a 4MB limit.
    bodyParser: false,
    externalResolver: true, // Prevents Next.js warning about unresolved promises
    responseLimit: false, // Disable Next.js 4MB API response size limit warnings for file downloads
  },
};

export default function handler(req: IncomingMessage, res: ServerResponse) {
  const backendUrl = process.env.BACKEND_URL || 'http://localhost:8080';

  // Extract path and query parameters from the incoming request URL
  const url = new URL(req.url || '', 'http://localhost');
  const path = url.pathname.replace(/^\/api/, '');
  const search = url.search;

  const targetUrl = new URL(`${backendUrl}${path}${search}`);

  // Forward incoming headers, omitting host and standard hop-by-hop headers
  const headers = { ...req.headers };
  delete headers.host;
  delete headers.connection;
  delete headers.keepalive;
  delete headers.te;
  delete headers.upgrade;

  const proxyReq = http.request(
    {
      hostname: targetUrl.hostname,
      port: targetUrl.port || (targetUrl.protocol === 'https:' ? 443 : 80),
      path: `${targetUrl.pathname}${targetUrl.search}`,
      method: req.method,
      headers,
    },
    (proxyRes) => {
      // Forward the backend status code and response headers
      res.writeHead(proxyRes.statusCode || 200, proxyRes.headers);
      
      // Standard high-reliability, maximum-performance native streaming pipe.
      // Automatically handles backpressure with 0MB RAM bloat at maximum C++ network speeds!
      proxyRes.pipe(res);
    }
  );

  proxyReq.on('error', (err) => {
    console.error(`[Proxy Error] Failed to connect to backend at ${targetUrl}:`, err);
    res.writeHead(502, { 'Content-Type': 'application/json' });
    res.end(
      JSON.stringify({
        error: 'Gagal menghubungi backend server',
        code: 'BACKEND_ERROR',
      })
    );
  });

  // Pipe the raw request stream directly into the backend request
  req.pipe(proxyReq);
}

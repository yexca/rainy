# Reverse Proxy and HTTPS

Plain `http://<nas-ip>:7650` works for playback on a home network. You need
HTTPS when you:

- access Rainy over the internet or any network you do not trust, or
- want to install the web app as a PWA. Browsers only enable service workers,
  and therefore installation and offline covers, on HTTPS or `localhost`.

Rainy must be served from the root of a host name, such as
`https://music.example.com`. Serving it under a sub-path such as `/rainy/` is
not supported.

## Requirements for Any Proxy

- Set `RAINY_TRUST_PROXY=true` so sign-in rate limiting sees the real client
  address. Rainy then trusts `X-Real-IP`, or the last `X-Forwarded-For` hop,
  which is the one your proxy appends. Enable it only when every request
  reaches Rainy through that proxy.
- Pass `X-Forwarded-Proto` so Rainy marks the session cookie `Secure`.
- Do not buffer `/api/events`. It is a long-lived server-sent event stream for
  scan progress and live updates; buffering freezes progress bars.
- Raise the request body limit so uploads of albums and lossless files work.
- Allow long read timeouts for streaming and transcoding.

## nginx

```nginx
server {
    listen 443 ssl;
    http2 on;
    server_name music.example.com;

    ssl_certificate     /etc/letsencrypt/live/music.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/music.example.com/privkey.pem;

    client_max_body_size 2g;          # uploads of albums and lossless files

    location / {
        proxy_pass http://127.0.0.1:7650;
        proxy_http_version 1.1;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 1h;            # long playback and transcoding
        proxy_send_timeout 1h;
    }

    # Server-sent events: never buffer
    location /api/events {
        proxy_pass http://127.0.0.1:7650;
        proxy_http_version 1.1;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection        "";
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 1d;
    }
}
```

Rainy also sends `X-Accel-Buffering: no` on the event stream, which disables
nginx buffering for that response even without the dedicated `location`.

## Caddy

```caddyfile
music.example.com {
    reverse_proxy 127.0.0.1:7650 {
        flush_interval -1
    }
}
```

Caddy obtains certificates, sets the `X-Forwarded-*` headers, and does not
limit request bodies by default. `flush_interval -1` streams responses
immediately.

## Synology DSM

In **Control Panel → Login Portal → Advanced → Reverse Proxy**, create a rule:

- Source: `HTTPS`, host name `music.example.com`, port `443`.
- Destination: `HTTP`, `localhost`, port `7650`.

Request a Let's Encrypt certificate under **Control Panel → Security →
Certificate** and assign it to the rule. DSM's proxy is based on nginx, so the
`X-Accel-Buffering: no` header keeps the event stream unbuffered. If large
uploads fail through the proxy because of its body size limit, upload through
the LAN address `http://<nas-ip>:7650` instead.

## Other Options

- **Tailscale:** `tailscale serve --bg 7650` gives Rainy an HTTPS `*.ts.net`
  address reachable only from your own devices, without opening a port.
- **Cloudflare Tunnel:** the free plan limits a single upload request to
  100 MB.

## After Enabling HTTPS

- Restart the container after setting `RAINY_TRUST_PROXY=true`
  (`docker compose up -d`).
- Install the app from the HTTPS address; see
  [Playback](../user/playback.md#install-the-app).
- In Subsonic clients, use the HTTPS address without `/rest`.

## Related Docs

- [Deployment security](security.md)
- [Configuration](configuration.md)
- [Troubleshooting](troubleshooting.md)

# Local development

## Backend with Docker

Start Docker Desktop first, then build and run the complete stack from the
`msqp` directory:

```powershell
.\build_app_image.bat
docker compose -f docker-compose-app.yml up -d
```

Expected host ports:

- Gate HTTP API: `13000`
- Connector Pomelo socket: `12000`
- User gRPC: `11500`
- NATS: `4222`
- Etcd: `2379`
- MongoDB: `27018`
- Redis: `6379`

Check the public entry points after the containers become healthy:

```powershell
Invoke-WebRequest -Method Post -ContentType application/json -Body '{}' http://127.0.0.1:13000/login
Test-NetConnection 127.0.0.1 -Port 12000
```

An empty login request should return a JSON error response. It proves that the
Gate route is reachable without creating or changing an account.

## Cocos Creator client

Open `qpClient-cocos388-migration` with Creator 3.8.8 and preview
`assets/Scenes/Main.scene`. The login shell uses `http://127.0.0.1:13000` and
the Gate response supplies the Connector address. For the local Docker stack,
that address is `127.0.0.1:12000`.

## Native backend configuration

The root `application.yml` files use host-loopback addresses and the MongoDB
password `root123456`. The `docker/app` files use Compose service names and the
container MongoDB password. Do not mix the two configuration sets.

# QQ status bot

Receives OneBot 11 group events from NapCat and replies to `状态检查` with a PNG generated from the existing Portal channel-monitor API.

1. Copy `.env.example` to `.env` and fill in the site account and allowed QQ group IDs.
2. Enable the matching local WebSocket server in NapCat.
3. Run `npm install`, `npm test`, then `npm start`.

Use a dedicated ordinary Portal user that can open the channel-status page. Do not use an administrator account. Keep NapCat and this process running on the same machine.

## Docker deployment

The compose file keeps NapCat's WebUI on host loopback port `16099`; OneBot port `3001` is available only inside the Compose network. Persist `data/napcat/config` and `data/napcat/qq` so a container recreation does not discard the QQ login.

Start NapCat first, scan the login QR, then start the bot:

```sh
docker compose up -d napcat
docker compose up -d --build status-bot
```

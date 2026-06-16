# mcskins

Eine schnelle, schlanke Minecraft-Skin-API in Go — **keine externen Dependencies**, nur die Standard-Library.

Sie löst Minecraft-Benutzernamen oder UUIDs über die offiziellen Mojang-APIs auf, holt die Skin-Textur und rendert daraus Avatare (Gesicht, Kopf mit Hut-Layer, Ganzkörper). Antworten werden zwischengespeichert (TTL-Cache), damit wiederholte Anfragen sofort beantwortet werden und Mojang geschont wird.

## Endpoints

| Methode & Pfad              | Beschreibung                                        |
| --------------------------- | --------------------------------------------------- |
| `GET /skin/{player}`        | Rohe Skin-Textur (PNG, 64×64)                       |
| `GET /face/{player}`        | Gesicht (8×8 Region), skaliert                      |
| `GET /head/{player}`        | Gesicht inkl. Hut-/Overlay-Layer                    |
| `GET /avatar/{player}`      | Alias für `/head`                                   |
| `GET /body/{player}`        | Flacher Ganzkörper-Render von vorne                 |
| `GET /health`               | Healthcheck (`{"status":"ok"}`)                     |
| `GET /`                     | API-Übersicht als JSON                              |

- `{player}` ist ein **Benutzername oder eine UUID** (mit oder ohne Bindestriche).
- Bildgröße per Query: `?size=N` (1–512, Standard 128). Skaliert mit Nearest-Neighbor → scharfe Pixel.
- Spieler ohne eigenen Skin bekommen einen deterministischen Fallback-Skin (kein Fehler).

### Beispiele

```bash
curl localhost:8080/face/Notch?size=256 -o notch.png
curl localhost:8080/head/jeb_         -o jeb.png
curl localhost:8080/body/Notch        -o notch_body.png
curl localhost:8080/skin/069a79f4-44e9-4726-a5be-fca90e38aaf5 -o skin.png
```

## Starten

```bash
go run ./cmd/mcskins
# oder
go build -o mcskins ./cmd/mcskins && ./mcskins
```

### Konfiguration (Umgebungsvariablen)

| Variable                     | Standard | Bedeutung                          |
| ---------------------------- | -------- | ---------------------------------- |
| `MCSKINS_ADDR`               | `:8080`  | Listen-Adresse                     |
| `MCSKINS_CACHE_TTL_SECONDS`  | `1800`   | Cache-Lebensdauer in Sekunden      |

## Docker

```bash
docker build -t mcskins .
docker run -p 8080:8080 mcskins
```

## Tests

```bash
go test ./...
```

## Architektur

```
cmd/mcskins        Einstiegspunkt, Server-Lifecycle, Graceful Shutdown
internal/server    HTTP-Routing, Handler, Fehler-Mapping, Logging
internal/mojang    Mojang-Client: Name→UUID→Textur, Caching, Fallback-Skin
internal/render    Ausschneiden & Skalieren von Skin-Regionen zu PNGs
internal/cache     Generischer, thread-sicherer TTL-Cache
```

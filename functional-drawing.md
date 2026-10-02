```
┌──────────────────────────────────────────────────────────────┐
│                     YOUR MACHINE (macOS)                      │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│   wp-perf scan mysite                                 │
│        │                                                     │
│        ▼                                                     │
│   ┌────────────────────┐     ~/.wp-perf/config.json          │
│   │  main.go           │◄─── target: ssh / docker / path     │
│   │  resolve target    │                                     │
│   │  production guard  │                                     │
│   └─────────┬──────────┘                                     │
│             │                                                │
│             ▼                                                │
│   ┌────────────────────────────────────────────────────┐     │
│   │                    SCANNERS                         │     │
│   │  database   media   bootstrap   pages   frontend    │     │
│   └──────┬─────────┬────────┬─────────┬────────┬───────┘     │
│          │         │        │         │        │             │
│          ▼         ▼        ▼         ▼        │ HTTP: weigh │
│   ┌────────────────────────────────────────┐  │ images, CSS │
│   │  wpcli.Runner                          │  │ and JS      │
│   │  embedded PHP probe → stdin            │  │             │
│   └───────────────────┬────────────────────┘  │             │
└───────────────────────┼───────────────────────┼─────────────┘
                        │ ssh (optional)        │
                        ▼                       ▼
┌──────────────────────────────────────────────────────────────┐
│                SITE'S MACHINE (VPS / Local / host)            │
├──────────────────────────────────────────────────────────────┤
│   [docker exec -i wordpress-container]  (optional)            │
│        │                                                     │
│        ▼                                                     │
│   php -d opcache… wp eval-file -   ← probe PHP on stdin       │
│        │                                                     │
│        ▼                                                     │
│   ┌────────────────────────────────────────────┐             │
│   │  WordPress (the site's own code)           │             │
│   │  $wpdb · object cache · theme · plugins    │             │
│   └───────────────────┬────────────────────────┘             │
│                       │                                      │
│                       ▼                                      │
│          "@@WPPERF@@" + JSON on stdout                       │
└───────────────────────┼──────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────────┐
│   decode JSON → finding functions (thresholds) → []Finding    │
│                         │                                     │
│            ┌────────────┴────────────┐                        │
│            ▼                         ▼                        │
│     terminal report             --json report                 │
│     (High → Medium → Low)       (findings + raw measurements) │
└──────────────────────────────────────────────────────────────┘

check-archive file.wpress  →  archive.Check (local, header walk, no extraction)
```

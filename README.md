┌─────────────────────────────────────────────────────────────┐
│                    Polymarket Data API                       │
│              (Leaderboard & Market Data)                     │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ↓ Every 10 minutes
┌─────────────────────────────────────────────────────────────┐
│                  Ingestion Service                           │
│  - Fetches top traders by PnL                                │
│  - Syncs to local database                                   │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ↓ Stores top trader addresses
┌─────────────────────────────────────────────────────────────┐
│                  SQLite Database                             │
│  - Users & Shares                                            │
│  - Top Traders List                                          │
│  - Trade Signals                                             │
│  - Positions & Trades                                        │
└─────────────────────┬───────────────────────────────────────┘
                      ↑
                      │ Queries top traders
┌─────────────────────┴───────────────────────────────────────┐
│              Blockchain Event Listener                       │
│  - Listens to CTF Exchange OrderFilled events                │
│  - Filters for top trader addresses                          │
│  - Creates trade signals for $10 copy trades                 │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ↓ Trade signals
┌─────────────────────────────────────────────────────────────┐
│                  Execution Engine                            │
│  - Picks up trade signals every 5s                           │
│  - Executes $10 copy trades via vault contract               │
│  - Tracks positions & PnL                                    │
└─────────────────────────────────────────────────────────────┘
                      │
                      ↓ Transactions
┌─────────────────────────────────────────────────────────────┐
│              Vault Smart Contract (Polygon)                  │
│  - Holds user USDC deposits                                  │
│  - Manages shares (ERC20-like)                               │
│  - Admin can execute trades but NOT withdraw                 │
└─────────────────────────────────────────────────────────────┘
                      │
                      ↓ Trade calls
┌─────────────────────────────────────────────────────────────┐
│         Polymarket CTF Exchange (Polygon)                    │
│  - Executes trades on prediction markets                     │
│  - Emits OrderFilled events                                  │
└─────────────────────────────────────────────────────────────┘
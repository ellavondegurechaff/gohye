# Item System Initialization

## Overview
The item system tables are now automatically initialized when the bot starts up.

## What happens on startup:

1. **Table Creation**: When the bot starts, it automatically creates the following tables if they don't exist:
   - `items` - Stores item definitions (broken disc, microphone, forgotten song)
   - `user_items` - Stores which items users own and their quantities

2. **Index Creation**: The following indexes are created for performance:
   - `idx_user_items_user_id` - Fast lookups of user's items
   - `idx_items_type` - Fast filtering by item type

3. **Initial Data**: The three crafting materials are automatically inserted:
   - `broken_disc` - 💿 Broken Disc (rarity 3)
   - `microphone` - 🎤 Microphone (rarity 3)
   - `forgotten_song` - 📜 Forgotten Song (rarity 3)

## Implementation Details:

- Tables are created using Bun ORM's auto-migration in `db.InitializeSchema()`
- Initial item data is inserted using `db.InitializeItemData()`
- Uses `ON CONFLICT DO NOTHING` to prevent duplicate inserts on restart
- All operations are logged for debugging

## No Manual Setup Required!
Simply start the bot and the item system will be ready to use:
- Players can earn items through `/work`
- View items in `/inventory` under Materials
- Fuse items with `/fuse` to create album cards

The initialization happens automatically every time the bot starts, ensuring the database is always in the correct state.
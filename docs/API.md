# API reference (v1)

Base path: `/api/v1`. JSON everywhere (UTF-8). Authenticated calls send `Authorization: Bearer <accessToken>`.
The same API serves the website and the future mobile app.

## Errors

```json
{ "error": { "code": "validation_failed", "message": "…", "fields": { "phone": "…", "items[0].quantity": "…" } } }
```

| Status | Meaning |
|---|---|
| 400 | malformed request (`invalid_json`, `bad_multipart`) |
| 401 | missing/expired token |
| 403 | role not allowed, or account blocked |
| 404 | not found (also used for other people's orders/receipts) |
| 409 | conflict: `out_of_stock`, `product_unavailable`, `status_changed`, `invalid_transition`, `already_claimed`, `category_not_empty` |
| 413 | file too large |
| 422 | validation failed (`fields` has per-field messages) |
| 429 | rate limited |

Lists are paginated: `?page=1&limit=20` → `{ items, page, limit, total }`.
Money is an integer in so'm. Timestamps are RFC 3339 (UTC).

## Telegram login (web and mobile)

1. `POST /auth/telegram/start` → `{ code, secret, deepLink, expiresAt }` (valid 5 min). Keep `secret` on the device.
2. Open `deepLink` (`https://t.me/<bot>?start=login_<code>`). The user taps **Start**; the bot confirms.
3. Poll `POST /auth/telegram/poll {code, secret}` every ~2 s → `{status: "pending"}` … then `{status: "confirmed", tokens: {accessToken, refreshToken, expiresIn}, user}` (returned **once**); `expired` otherwise.
4. Access tokens live 15 min. `POST /auth/refresh {refreshToken}` rotates the pair; replaying an old refresh token revokes the whole session family. `POST /auth/logout {refreshToken}` → 204.

## Public

| | |
|---|---|
| `GET /health` | `{status, db}` |
| `GET /settings/public` | `{shopName, shopPhone}` |
| `GET /categories` | `{items: [{id, name, slug, productCount, image}]}` |
| `GET /products` | `?q=&category=<slug>&sort=new\|price_asc\|price_desc\|name&featured=1&new=1&page=&limit=` → cards `{id, slug, name, brand, category, priceFrom, oldPriceFrom, inStock, hasVariants, isNew, isFeatured, image{thumb,card,full}}` |
| `GET /products/{slug}` | detail: `description`/`ingredients`/`howToUse` (Markdown, render **without** raw HTML), `optionLabel`, `images[]`, `variants[{id, label, price, oldPrice, inStock, image}]` |

## Customer (login required)

| | |
|---|---|
| `GET /me`, `PATCH /me` | profile; `{phone?, notificationsEnabled?}` |
| `GET /payment-info` | `{cardNumber, cardHolder, paymentNote, shopPhone}`: show on checkout |
| `POST /uploads/receipt` | multipart field `file` (JPG/PNG/WebP, ≤10 MB, ≥200 px) → `{id}` |
| `POST /orders` | see below → order |
| `GET /orders`, `GET /orders/{id}` | own orders with `timeline` |
| `GET /files/receipts/{id}` | the receipt image (owner or staff only; send the Bearer header) |

```json
POST /orders
{
  "items": [{ "variantId": "…", "quantity": 2 }],
  "customerName": "Ali Valiyev",
  "phone": "+998 90 123 45 67",
  "addressText": "Toshkent, Chilonzor 5-kvartal",
  "lat": 41.31, "lng": 69.24,
  "receiptFileId": "<id from /uploads/receipt>",
  "comment": ""
}
```

The server computes prices and the total, reserves stock and returns the order (`status: "waiting"`). Name, phone, address, coordinates and receipt are all required. A receipt can be used for one order only.

## Admin (role `admin` or `owner`)

| | |
|---|---|
| `GET /admin/stats` | `{counts: {waiting, confirmed, shipping, delivered, rejected}, unassigned}` (poll for the notification badge) |
| `GET /admin/orders` | `?status=&assigned=me\|none&q=<number or phone or name>` |
| `GET /admin/orders/{id}` | full view: customer, receipt id (`/files/receipts/{receiptFileId}`), `assignedAdmin`, `receiptDuplicate`, `history`, `notes` |
| `POST /admin/orders/{id}/claim` | "Men olaman"; owner may send `{force: true}` to reassign |
| `POST /admin/orders/{id}/status` | `{to, expectedStatus, comment?, rejectReason?}`; `rejectReason` required for `rejected` |
| `POST /admin/orders/{id}/notes` | `{body}` |
| `GET/POST /admin/categories`, `PUT/DELETE /admin/categories/{id}` | `{name, imageId?, position, isActive}` |
| `GET/POST /admin/products`, `GET/PUT /admin/products/{id}`, `DELETE` (= archive) | see below |
| `POST /admin/uploads/image?kind=product\|category` | multipart `file` → `{id, urls}` (stored in 3 sizes) |
| `GET /admin/settings` | shop and payment settings |

Product body (create/update): `{categoryId, name, brand, description, ingredients, howToUse, optionLabel, status: draft|active|archived, isNew, isFeatured, imageIds: [≤8 uploaded image ids; first = main], variants: [{id?, label, sku, price, oldPrice?, stock, imageId?, isActive}]}`.
Send existing variant `id`s to update them; variants left out are removed (or deactivated if orders reference them). With more than one variant every `label` is required and unique. `active` products need at least one image and one active variant.

### Owner only

`PUT /admin/settings` (`{shopName, shopPhone, cardNumber, cardHolder, paymentNote}`, partial), `GET /admin/admins`, `POST /admin/admins/invites` (one-time Telegram link, 24 h), `PATCH /admin/admins/{id}` (`{isActive}`), `DELETE /admin/admins/{id}` (back to customer), `GET /admin/audit`.

## Order statuses

`waiting → confirmed | shipping | rejected`, `confirmed → shipping | rejected`, `shipping → delivered`. `delivered` and `rejected` are final.

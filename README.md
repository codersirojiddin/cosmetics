# Cosmetics: onlayn kosmetika do'koni

Telegram orqali kirish, mahsulot tanlash, **chek rasmini yuklab** buyurtma berish va admin tomonidan tekshirib yuborish.
To'lov tizimi yo'q: mijoz kartaga pul o'tkazadi va chekni yuklaydi, admin tekshiradi.

**Holat:** backend (API + Telegram bot) tayyor va testlangan. Admin panel va mijoz sayti (`apps/web`) keyingi bosqich.

```
apps/api     Go backend (chi + pgx + Telegram bot), web va mobile uchun bitta API
apps/web     (keyingi bosqich) Next.js: mijoz sayti + /admin panel
docs/API.md  API qo'llanmasi
render.yaml  Render uchun Blueprint
```

## Tezkor ishga tushirish (lokal)

Kerak: Go 1.26+, PostgreSQL 16.

```bash
cd apps/api
cp .env.example .env          # JWT_SECRET ni to'ldiring: openssl rand -hex 32
createdb cosmetics_dev        # va DATABASE_URL ni moslang
make migrate                  # bazani yaratadi
make seed                     # demo kategoriya va mahsulotlar (faqat dev)
make run                      # http://localhost:8080  (GET /api/v1/health)
make test                     # TEST_DATABASE_URL nomida "_test" bo'lgan bazani talab qiladi
```

Testlar `cosmetics_test` bazasini **to'liq tozalaydi**, shuning uchun nomida `_test` bo'lmagan bazaga ulanishni rad etadi.

## Telegram bot

1. Telegram'da **@BotFather** → `/newbot` → token oling → `TELEGRAM_BOT_TOKEN`.
2. Lokal: `BOT_MODE=polling` (tunnel kerak emas). Production: `BOT_MODE=webhook` (server o'zi webhook'ni ro'yxatdan o'tkazadi).
3. **Owner bo'lish:** botga `/id` yozing, chiqqan raqamni `OWNER_TELEGRAM_IDS` ga qo'ying. Shu akkaunt saytga kirganda `owner` bo'ladi.
4. **Admin qo'shish:** owner admin panelda "Admin qo'shish" ni bosadi, bir martalik havola (24 soat) chiqadi. Xodim uni Telegram'da ochib "Start" bosadi.
5. Yangi buyurtma kelganda hamma adminga bot xabar yuboradi. **«Men olaman»** tugmasini bosgan admin buyurtmaga biriktiriladi, boshqalarning xabari yangilanadi.

## Deploy: Neon + Render + Cloudflare R2

1. **Neon** (Frankfurt): `DATABASE_URL` = *pooled* manzil (`...-pooler...`), `DIRECT_URL` = shu manzildan `-pooler` ni olib tashlangani. `channel_binding=require` ni ilova o'zi e'tiborsiz qoldiradi.
2. **Cloudflare R2:** ikkita bucket oching: `public` (mahsulot rasmlari; ochiq domen yoqing → `S3_PUBLIC_BASE_URL`) va `private` (cheklar; hech qachon ochiq qilmang). Ikkala bucketga yozish huquqli API kalit yarating.
3. **Render:** New → Blueprint → shu repo. `render.yaml` o'qiladi, maxfiy qiymatlarni dashboard'da kiritasiz. Birinchi deploydan keyin servis URL'ini `PUBLIC_API_URL` ga yozib qayta deploy qiling.
4. Migratsiyalar har ishga tushganda avtomatik qo'llanadi (`AUTO_MIGRATE=true`, bitta instance).
5. Render'ning **bepul** rejasi 15 daqiqa harakatsizlikdan keyin uxlaydi: test uchun yaxshi, do'kon uchun `starter`.

`main` ga push → GitHub Actions testlarni ishlatadi → Render avtomatik deploy qiladi.

### Muhit o'zgaruvchilari

Hammasi `apps/api/.env.example` da izohlangan. Asosiylari: `DATABASE_URL`, `DIRECT_URL`, `JWT_SECRET` (≥32 belgi), `TELEGRAM_BOT_TOKEN`, `BOT_MODE`, `OWNER_TELEGRAM_IDS`, `PUBLIC_API_URL`, `WEB_URL`, `CORS_ORIGINS` (production'da majburiy), `STORAGE_DRIVER` (+ `S3_*`).

## Buyurtma oqimi

```
waiting ──► confirmed ──► shipping ──► delivered
   │            │  ▲
   │            │  └── waiting → shipping ham mumkin (qo'ng'iroqsiz darrov jo'natish)
   └────────────┴──► rejected (sabab majburiy, qoldiq omborga qaytadi)
```

- Server noto'g'ri o'tishlarni rad etadi; har o'zgarish tarixga yoziladi (kim, qachon, izoh).
- Admin ekrani eskirgan bo'lsa (`expectedStatus` mos kelmasa) o'zgartirish rad etiladi: ikki admin bir-birini bosib yubormaydi.
- Biriktirilgan buyurtmani faqat o'sha admin yoki owner o'zgartira oladi.
- Har status o'zgarganda mijozga Telegram'da xabar boradi.

## Xavfsizlik

- Chek rasmlari **private** saqlanadi, faqat buyurtma egasi va adminlar ko'ra oladi.
- Bir xil yoki juda o'xshash chek ikkinchi marta yuklansa, adminga ogohlantirish chiqadi.
- Narx va jami summa doim bazadan hisoblanadi (mijoz yuborgan summaga ishonilmaydi).
- Rol har so'rovda bazadan tekshiriladi: bloklangan admin darhol kirolmaydi.
- Karta raqamini va adminlarni faqat `owner` o'zgartira oladi.
- **Maxfiy qiymatlarni (bot token, baza paroli) hech qachon chatga yoki repoga yozmang.** Agar yozilgan bo'lsa, almashtiring: BotFather → `/revoke`, Neon → parolni reset qiling.

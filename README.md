# Job Finder → Telegram (Modular Pipeline)

سرویس ماژولار که جاب‌های Remote/Relocation باکیفیت را از چند منبع پیدا و در تلگرام منتشر می‌کند.

## معماری

```
Providers → Normalize → Merge → Dedup → Filter → Publish(Telegram)
```

هر منبع یک `Provider`، هر فیلتر یک `Filter`، ذخیره‌سازی یک `SeenStore`، انتشار یک `Publisher`.
افزودن منبع جدید = یک فولدر در `internal/providers/` + یک خط blank-import در `cmd/jobfinder/main.go`.
پروژه فقط با کتابخانه‌ی استاندارد Go ساخته شده (بدون وابستگی خارجی).

## پیش‌نیازها

- کلید RapidAPI برای JSearch و/یا یک LinkedIn Jobs API.
- بات تلگرام (BotFather) + کانال (بات ادمین باشد).

## اجرای محلی

```bash
cp .env.example .env          # رازها را پر کن
set -a && source .env && set +a
go run ./cmd/jobfinder        # از config.json می‌خواند
```

## تنظیمات

- **رازها** فقط در `.env` / GitHub Secrets: `RAPIDAPI_KEY`, `LINKEDIN_RAPIDAPI_KEY`,
  `LINKEDIN_RAPIDAPI_HOST`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`.
- **فیلترها و منابع** در `config.json` (فایل `config.example.json` نمونه است):
  `providers`, `keywords`, `countries`, `remoteOnly`, `relocationOnly`,
  `postedWithinHours`, `companyBlacklist`, ...

پیش‌فرض `config.json` فقط `jsearch` را فعال دارد. برای فعال‌کردن لینکدین، `"linkedin"` را
به `providers` اضافه کن و رازهای مربوطه را ست کن.

## اجرای خودکار

`.github/workflows/daily.yml` روزی یک بار اجرا می‌شود (قابل‌تنظیم) و وضعیت را commit می‌کند.
Secretها را در Settings → Secrets and variables → Actions ثبت کن.

## ساختار

```
cmd/jobfinder/        Composition Root (DI)
internal/core/        مدل دامنه
internal/config/      خواندن env + config.json
internal/providers/   Provider interface + linkedin, jsearch
internal/normalize/   نرمال‌سازی
internal/dedup/       تشخیص تکراری
internal/filter/      زنجیره‌ی فیلتر قانون‌محور
internal/store/       SeenStore (فایلی)
internal/publisher/   Publisher + telegram
internal/pipeline/    ارکستراسیون
internal/retry/       backoff نمایی
internal/ratelimit/   محدودیت نرخ
```

## فازهای بعدی

- فاز ۲: فیلتر AI با Claude (پشت همان `Filter`).
- فاز ۳: Providerهای Greenhouse / Lever / Ashby / Workday.
- فاز ۴: ذخیره‌سازی Upstash Redis (پشت همان `SeenStore`).

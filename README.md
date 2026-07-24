# Job Finder → Telegram

هر روز جاب‌های جدید برنامه‌نویسی را از JSearch می‌گیرد و به یک کانال تلگرام می‌فرستد.

## پیش‌نیازها

1. **کلید RapidAPI (JSearch):** در https://rapidapi.com ثبت‌نام کن، به JSearch مشترک شو
   (پلن رایگان برای شروع کافی است) و کلید `X-RapidAPI-Key` را بردار.
2. **بات تلگرام:** در تلگرام به `@BotFather` پیام بده، دستور `/newbot` را بزن،
   نام و یوزرنیم بده و توکن را بردار.
3. **کانال تلگرام:** یک کانال بساز، بات را به‌عنوان ادمین اضافه کن،
   و آیدی کانال (`@yourchannel`) را بردار.

## اجرای محلی

```bash
cp .env.example .env
# .env را با مقادیر واقعی پر کن
set -a && source .env && set +a
go run ./cmd/jobfinder
```

## اجرای خودکار (GitHub Actions)

سه مقدار زیر را در Settings → Secrets and variables → Actions مخزن ثبت کن:

- `RAPIDAPI_KEY`
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_CHAT_ID`

workflow هر روز ساعت ۰۶:۰۰ UTC اجرا می‌شود و می‌توان از تب Actions دستی هم اجرایش کرد.

## تنظیمات (متغیرهای محیطی اختیاری)

| متغیر | پیش‌فرض | توضیح |
|-------|---------|-------|
| `KEYWORDS` | چند عنوان رایج | کلمات کلیدی جست‌وجو، جداشده با کاما |
| `DATE_POSTED` | `today` | `all` \| `today` \| `3days` \| `week` \| `month` |
| `MAX_PER_RUN` | `20` | سقف پیام در هر اجرا |
| `DELAY_SECONDS` | `4` | فاصله بین پیام‌ها |
| `SUMMARY_RUNES` | `300` | حداکثر طول توضیح |

## محدودیت‌ها

- پلن رایگان JSearch درخواست محدودی دارد؛ برای حجم بالاتر پلن پولی لازم است.
- خلاصه فارسی در فاز بعدی با Claude API اضافه می‌شود.

## ساختار

```
cmd/jobfinder/    نقطه شروع (هماهنگ‌کننده)
internal/config/  خواندن تنظیمات از محیط
internal/jsearch/ گرفتن جاب‌ها از JSearch
internal/store/   ضدتکرار (جاب‌های دیده‌شده)
internal/message/ ساخت متن پیام فارسی
internal/summarize/ کوتاه‌سازی توضیح
internal/telegram/  ارسال به کانال
data/seen_jobs.json وضعیت ذخیره‌شده
```

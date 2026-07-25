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

### پوشش جغرافیایی

فهرست `countries` هم به عنوان پارامتر جست‌وجو به API فرستاده می‌شود و هم به‌عنوان
فیلتر روی نتیجه اعمال می‌گردد. کد دو‌حرفی ISO بدهید. فهرست خالی یعنی
جست‌وجوی بی‌قید که عملاً نتیجه‌ی آمریکامحور می‌دهد.

### چند کلید API و سوییچ خودکار

کلیدها با کاما در یک متغیر محیطی می‌آیند:

```
RAPIDAPI_KEYS=key1,key2,key3,key4
```

وقتی یک کلید سهمیه‌اش تمام شود (پاسخ ۴۲۹ با پیام quota) یا اشتراکش نامعتبر
باشد (۴۰۳)، همان‌جا کنار گذاشته و کلید بعدی جایگزین می‌شود. کلیدِ کنارگذاشته
تا پایان آن اجرا دوباره امتحان نمی‌شود. پاسخ ۴۲۹ بدون پیام quota یعنی سقف
لحظه‌ای است و با همان کلید دوباره تلاش می‌شود.
متغیر تک‌کلیدی `RAPIDAPI_KEY` هم برای سازگاری پشتیبانی می‌شود.

### چرخش کشورها بین اجراها

به‌جای یک اجرای سنگین روزانه، کشورها بین اجراهای پیاپی پخش می‌شوند:

```json
"rotation": { "slotHours": 4, "countriesPerRun": 1 }
```

هر چهار ساعت یک اجرا که فقط یک کشور را می‌گیرد. انتخاب کشور فقط تابع زمان
است نه وضعیت ذخیره‌شده، پس از دست رفتن یک اجرا چرخه را به‌هم نمی‌ریزد.
با `slotHours: 0` چرخش خاموش می‌شود و هر اجرا همه‌ی کشورها را می‌گیرد.

`postedWithinHours` باید از طول یک دوره‌ی کامل بیشتر باشد، وگرنه آگهی‌های
وسط فاصله از دست می‌روند:

```
دوره کامل = (تعداد کشور ÷ countriesPerRun) × slotHours
10 کشور، هر ۴ ساعت یکی  →  40 ساعت  →  postedWithinHours: 96 ✓
```


### پوشش واقعی JSearch

اندازه‌گیری مستقیم مقابل API در ۲۰۲۶-۰۷-۲۵ نشان داد این کشورها با هر
پرس‌وجو و هر بازه زمانی **صفر** نتیجه می‌دهند:

```
IE  NL  SE  DK  NO  FI  BE  PL  CZ  AU
```

این محدودیت پوشش خود منبع است نه تنظیمات. گذاشتنشان در فهرست فقط سهمیه
می‌سوزاند. کشورهایی که واقعا نتیجه می‌دهند:

```
US  GB  DE  CA  FR  ES  IT  PT  CH  AT
```

اگر استرالیا یا هلند لازم دارید، به یک منبع دیگر نیاز است.

### عبارت جست‌وجو در برابر کلیدواژه‌ی فیلتر

این دو عمدا جدا هستند:

```json
"searchQueries": ["remote developer", "remote engineer"],
"keywords":      ["developer", "engineer", "entwickler"]
```

`searchQueries` به API می‌رود. داشتن «remote» در متن پرس‌وجو بازده را از
صفر به چند ده آگهی می‌رساند، چون جست‌وجوی کشوری بدون آن بازار محلیِ
حضوری را برمی‌گرداند.

`keywords` فقط روی نتیجه فیلتر می‌کند و باید عام بماند. اگر عبارت‌های
جست‌وجو را اینجا بگذارید، «remote developer» به‌عنوان زیررشته‌ی اجباری
عنوان عمل می‌کند و تقریبا همه‌چیز را رد می‌کند.

`excludeKeywords` فقط روی عنوان اعمال می‌شود، نه متن توضیحات.

### مصرف سهمیه‌ی API

طرح Basic هر کلید ۲۰۰ درخواست در ماه است.

```
درخواست هر اجرا  = countriesPerRun × len(keywords) × numPages
درخواست ماهانه   = درخواست هر اجرا × تعداد اجرا در روز × ۳۰
```

با تنظیمات فعلی:

```
هر اجرا:  1 کشور × 4 عبارت × 1 صفحه  =   4 درخواست
روزانه:   6 اجرا × 4                    =  24 درخواست
ماهانه:   24 × 30                       = 720 درخواست
بودجه:    5 کلید × 200                  = 1000 درخواست  ✓

دوره کامل ۱۰ کشور = ۱۰ اسلات × ۴ ساعت = ۴۰ ساعت
```

### قید دورکاری یا جابه‌جایی

کلید `requireRemoteOrRelocation` آگهی حضوریِ بدون پشتیبانی ویزا یا جابه‌جایی را
رد می‌کند. اگر متقاضی خارج از کشور آگهی هستید روشن بگذارید. خاموش‌کردنش
آگهی‌های حضوری محلی را هم وارد می‌کند و تعداد را زیاد ولی کیفیت را کم می‌کند.

## گزارش پایان اجرا

کلید `summary` تعیین می‌کند گزارش پایان اجرا چه وقت برود:

```
onChange  (پیش‌فرض)  فقط وقتی آگهی منتشر شده یا خطایی رخ داده
always               هر اجرا. با شش اجرا در روز پرسروصداست.
never                هیچ‌وقت
```

پیام تعداد دریافت‌شده، تکراری‌حذف‌شده، تازه و منتشرشده را می‌گوید و خطای
منابع را هم گزارش می‌کند. با حالت پیش‌فرض، هر خطایی حتماً گزارش می‌شود؛
پس خرابی خاموش ممکن نیست.

## اجرای خودکار

`.github/workflows/daily.yml` هر چهار ساعت اجرا می‌شود (قابل‌تنظیم) و وضعیت را commit می‌کند.
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
internal/rotate/      چرخش کشورها بین اجراها
```

## فازهای بعدی

- فاز ۲: فیلتر AI با Claude (پشت همان `Filter`).
- فاز ۳: Providerهای Greenhouse / Lever / Ashby / Workday.
- فاز ۴: ذخیره‌سازی Upstash Redis (پشت همان `SeenStore`).

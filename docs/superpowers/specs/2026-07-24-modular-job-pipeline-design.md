# سند معماری: سرویس ماژولار پیداکردن و انتشار جاب‌ها

تاریخ: ۲۰۲۶-۰۷-۲۴
جایگزینِ توسعه‌یافته‌ی سند قبلی (`2026-07-24-linkedin-job-finder-design.md`).

## هدف

سرویس Production-Ready و ماژولار که به‌صورت خودکار بهترین آگهی‌های شغلی را از چند منبع
(اول LinkedIn و JSearch) پیدا می‌کند، فقط جاب‌های باکیفیتِ Remote یا Relocation را نگه
می‌دارد و در یک کانال تلگرام منتشر می‌کند. افزودن منبع جدید باید کمتر از ۵ دقیقه و بدون
تغییر بخش‌های دیگر ممکن باشد.

## تصمیم‌های کلیدی

| موضوع | تصمیم | دلیل |
|-------|-------|------|
| زبان | Go | ترجیح کاربر، باینری تکی، مناسب GitHub Actions |
| وابستگی خارجی | صفر (فقط stdlib) | شبکه‌ی کاربر به `proxy.golang.org` گوگل دسترسی ندارد؛ هر dep، بیلد محلی را می‌شکند. همه‌چیز با stdlib شدنی است |
| منبع اصلی | LinkedIn (RAPIDAPI، غیررسمی) پشت `Provider` | قابل‌تعویض |
| منبع دوم | JSearch پشت `Provider` | Fallback و پوشش گسترده‌تر |
| فیلتر | زنجیره‌ی قانون‌محور (فاز ۱) + AI با همان interface (فاز ۲) | ارزان‌ها اول، AI فقط روی باقی‌مانده‌ها |
| زمان‌بندی | روزی یک بار، قابل‌تنظیم | دوام روی پلن رایگان APIها |
| ذخیره‌سازی | `SeenStore` interface، پیش‌فرض فایلی؛ Upstash بعداً | ماژولار، در فرکانس روزانه فایل کافی است |
| لاگ | `log/slog` | ساختاریافته، stdlib |

## اصول معماری

- **Clean Architecture / Dependency Inversion:** `pipeline` فقط به interfaceها
  (`Provider`, `Filter`, `SeenStore`, `Publisher`) و مدل دامنه (`core`) وابسته است، نه به
  پیاده‌سازی‌های مشخص (لینکدین، تلگرام، ...).
- **SOLID:** هر پکیج یک مسئولیت؛ هیچ Provider به Provider دیگر وابسته نیست.
- **Dependency Injection:** ساخت و وصل‌کردن قطعات فقط در `cmd/jobfinder/main.go`
  (Composition Root) انجام می‌شود.

## ساختار پوشه

```
cmd/jobfinder/main.go        Composition Root (وصل‌کردن قطعات، DI)
internal/
  core/                      مدل دامنه: Job, Filters, Decision (بدون وابستگی)
  config/                    خواندن env (رازها) + فایل JSON (فیلترها/تنظیمات)
  providers/                 Provider interface + Registry + Build
    linkedin/                منبع لینکدین (RapidAPI، host/path/key از config)
    jsearch/                 منبع JSearch (ریفکتور کد فعلی)
  normalize/                 نرمال‌سازی مشترک خروجی همه Providerها
  dedup/                     تشخیص تکراری داخل یک اجرا
  filter/                    Filter interface + Chain + فیلترهای قانون‌محور
  store/                     SeenStore interface + پیاده‌سازی فایلی
  publisher/                 Publisher interface
    telegram/                پیاده‌سازی تلگرام + قالب پیام
  pipeline/                  ارکستراسیون مراحل
  retry/                     Exponential backoff
  ratelimit/                 محدودیت نرخ هر Provider
config.example.json          نمونه تنظیمات (غیرِراز)
.env.example                 نمونه رازها
.github/workflows/daily.yml  زمان‌بندی روزانه
```

## مدل دامنه (`internal/core`)

```go
type Job struct {
    ID             string
    Title          string
    Company        string
    Location       string
    Remote         bool
    Relocation     bool
    EmploymentType string    // FULLTIME | PARTTIME | CONTRACTOR | INTERN
    Seniority      string    // junior | mid | senior | lead
    Description    string
    URL            string
    PostedAt       time.Time
    Source         string    // نام Provider
}

type Filters struct {
    Countries         []string
    Keywords          []string
    ExcludeKeywords   []string
    EmploymentTypes   []string
    Seniority         []string
    RemoteOnly        bool
    RelocationOnly    bool
    PostedWithinHours int
    CompanyWhitelist  []string
    CompanyBlacklist  []string
}

type Decision struct {
    Publish bool
    Reason  string   // چرا رد/قبول شد (برای لاگ)
}

// Fingerprint کلید یکتای جاب برای dedup و SeenStore.
// اولویت: URL نرمال‌شده؛ در نبودش هش Company+Title.
func (j Job) Fingerprint() string
```

## قراردادها (Interfaces)

```go
// providers
type Provider interface {
    Name() string
    SearchJobs(ctx context.Context, f core.Filters) ([]core.Job, error)
}

// filter
type Filter interface {
    Name() string
    Evaluate(ctx context.Context, job core.Job) (core.Decision, error)
}

// store
type SeenStore interface {
    IsSeen(fingerprint string) bool
    MarkSeen(fingerprint string)
    Save(ctx context.Context) error
}

// publisher
type Publisher interface {
    Publish(ctx context.Context, job core.Job) error
}
```

### Registry افزودن Provider

```go
// internal/providers/registry.go
type Factory func(cfg core.ProviderConfig) (Provider, error)

var registry = map[string]Factory{}

func Register(name string, f Factory) { registry[name] = f }

func Build(names []string, cfg core.Config) ([]Provider, error) // فقط منابع فعال را می‌سازد
```

هر Provider در `init()` خودش را ثبت می‌کند؛ یک فایل جمع‌کننده آن‌ها را blank-import می‌کند.
**افزودن Provider جدید = یک فولدر جدید + یک خط blank-import.**

## Pipeline (`internal/pipeline`)

جریان:

```
Providers (موازی؛ هرکدام retry + rate-limit + timeout)
   ↓  خطای یک Provider فقط لاگ می‌شود؛ بقیه ادامه (Error Isolation)
Normalize
   ↓
Merge
   ↓
Dedup داخل‌اجرا (اولویت با ترتیب Providerها)
   ↓
Dedup ماندگار (حذف Fingerprintهای موجود در SeenStore)
   ↓
Filter Chain (رد با اولین فیلترِ ردکننده؛ دلیل لاگ می‌شود)
   ↓
Publish → فقط پس از انتشار موفق: MarkSeen
   ↓
Store.Save
```

توضیح تصمیم: فیلترهای ارزان و dedup قبل از AI اجرا می‌شوند تا در فاز ۲ فیلتر AI فقط روی
چند جاب باقی‌مانده اجرا شود. `MarkSeen` فقط پس از انتشار موفق است تا خطای ارسال منجر به
گم‌شدن جاب نشود (تحویل مطمئن).

## نرمال‌سازی (`internal/normalize`)

- Trim و جمع‌کردن فاصله‌ها در فیلدهای متنی.
- استانداردسازی `EmploymentType` و `Seniority` به واژگان ثابت.
- سیگنال اولیه‌ی `Remote`/`Relocation` از روی کلمات کلیدی توضیح (فیلتر بعداً دقیق‌تر می‌کند).

## Dedup داخل‌اجرا (`internal/dedup`)

حذف تکراری‌ها بر اساس، به‌ترتیب:
1. URL نرمال‌شده (حذف query/tracking، lowercase).
2. در نبودش: `Company + Title` نرمال‌شده.
3. تشابه: مقایسه‌ی توکن‌ستِ `Title+Company` با آستانه (برای آگهی‌های تقریباً یکسان).

اولین موردِ دیده‌شده نگه داشته می‌شود (اولویت با ترتیب Providerها در config؛ لینکدین اول).

## فیلترها (`internal/filter`)

Chain runner: جاب باید از همه رد شود؛ اولین `Publish=false` با دلیل لاگ و جاب حذف می‌شود.

فیلترهای قانون‌محور فاز ۱:
- **EmploymentType:** رد `INTERN` مگر `allowInternship` فعال باشد.
- **Keyword:** باید حداقل یکی از `Keywords` را داشته باشد؛ اگر `ExcludeKeywords` بود رد.
- **Company:** اگر Blacklist شامل شرکت بود رد؛ اگر Whitelist ناخالی بود و شرکت در آن نبود رد.
- **Freshness:** اگر `PostedAt` قدیمی‌تر از `PostedWithinHours` بود رد.
- **RemoteRelocation:** اگر `RemoteOnly` یا `RelocationOnly`، باید سیگنال قویِ واقعی باشد
  (`fully remote`, `work from anywhere`, `100% remote`, `remote-first` / `visa sponsorship`,
  `relocation package`)؛ سیگنال منفی (`hybrid`, `on-site`, `must be located in`,
  `no relocation`) رد می‌کند.
- **SpamQuality:** توضیح کوتاه‌تر از `minDescriptionRunes` یا خالی، رد.

فاز ۲ (AI): پکیج `internal/filter/ai` همین `Filter` را با Claude پیاده و به انتهای زنجیره
اضافه می‌شود؛ هیچ کد دیگری تغییر نمی‌کند.

## ذخیره‌سازی (`internal/store`)

- `SeenStore` interface؛ پیاده‌سازی فایلی `file.go` که فایل JSON از Fingerprintها را
  می‌خواند/می‌نویسد و فقط وقتی تغییر کرد `Save` می‌کند (تا commit بی‌مورد نشود).
- هرس آیدی‌های قدیمی برای جلوگیری از رشد بی‌نهایت.
- فاز ۴: `redis.go` با Upstash REST (روی `net/http`) پشت همان interface.

## Publisher (`internal/publisher/telegram`)

- ارسال با `sendMessage` بات، با فاصله‌ی زمانی بین پیام‌ها و سقف روزانه.
- قالب پیام (نمونه):

```
🚀 Senior Backend Engineer
🏢 Stripe
🌍 Remote (Worldwide)
✈️ Relocation: Yes
🕒 Posted: 3 hours ago
🔗 <لینک>
```

## Cross-cutting

- **Retry (`internal/retry`):** Exponential backoff فقط برای خطاهای موقت (5xx, timeout, 429).
- **Rate limit (`internal/ratelimit`):** محدودکننده‌ی ساده‌ی هر Provider.
- **Logging (`slog`):** هر مرحله لاگ ساختاریافته (تعداد گرفته‌شده/dedup/ردشده/ارسال‌شده و دلیل).
- **Config:** رازها فقط از env؛ فیلترها و تنظیمات از `config.json`.

### رازها (env)

```
RAPIDAPI_KEY               # JSearch
LINKEDIN_RAPIDAPI_KEY      # LinkedIn provider
LINKEDIN_RAPIDAPI_HOST     # host همان API لینکدین
TELEGRAM_BOT_TOKEN
TELEGRAM_CHAT_ID
# فاز ۲: ANTHROPIC_API_KEY
# فاز ۴: UPSTASH_REDIS_REST_URL, UPSTASH_REDIS_REST_TOKEN
```

### تنظیمات (`config.json`)

```json
{
  "providers": ["linkedin", "jsearch"],
  "filters": {
    "countries": ["US", "DE", "NL"],
    "keywords": ["developer", "engineer", "backend"],
    "excludeKeywords": ["sales", "manager"],
    "employmentTypes": ["FULLTIME", "CONTRACTOR"],
    "seniority": ["mid", "senior"],
    "remoteOnly": true,
    "relocationOnly": false,
    "postedWithinHours": 48,
    "companyWhitelist": [],
    "companyBlacklist": []
  },
  "maxPerRun": 20,
  "delaySeconds": 4,
  "allowInternship": false,
  "minDescriptionRunes": 200
}
```

## آزمایش (Testing)

- هر Provider با `httptest` (پاسخ جعلی → مدل `core.Job`).
- `dedup`, `filter`, `normalize`, `store` تست واحد.
- `pipeline` با Providerها/Publisher جعلی (fake) تست یکپارچه.

## فازبندی

- **فاز ۱ (این پیاده‌سازی):** اسکلت کامل + LinkedIn + JSearch + normalize + dedup + فیلتر
  قانون‌محور + store فایلی + publisher تلگرام + pipeline + config + retry + ratelimit +
  workflow روزانه. ریفکتور کد فعلی (jsearch/store/telegram/message) داخل این ساختار.
- **فاز ۲:** فیلتر AI با Claude.
- **فاز ۳:** Providerهای Greenhouse / Lever / Ashby / Workday.
- **فاز ۴:** ذخیره‌سازی Upstash Redis.

## خارج از محدوده

- اپلای خودکار به جاب‌ها.
- رابط وب/داشبورد.
- اسکرپ مستقیم لینکدین (به‌جایش API غیررسمی پشت Provider).

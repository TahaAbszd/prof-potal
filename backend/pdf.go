package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const resumeHTML = `<!doctype html><html lang="fa" dir="rtl"><head><meta charset="utf-8"><title>رزومه علمی {{.Professor.Name}} | دانشگاه هرمزگان</title><style>
{{.FontCSS}}
@page{size:A4;margin:0}
*{box-sizing:border-box}body{margin:0;font-family:IRANSans,Tahoma,sans-serif;direction:rtl;color:#20384b;font-size:10pt;line-height:1.75;-webkit-print-color-adjust:exact;print-color-adjust:exact}
.top{height:11mm;background:#153f62}.page{padding:13mm 18mm 16mm}.brand{font-size:10pt;color:#336787;font-weight:bold;letter-spacing:.1px}.brand small{font-size:8pt;color:#8394a0;font-weight:normal;margin-right:8px}.head{display:flex;justify-content:space-between;gap:12mm;border-bottom:1px solid #d5e1e9;padding:5mm 0 8mm}.head h1{font-size:23pt;line-height:1.5;color:#163a57;margin:0 0 1mm}.head .subtitle{color:#567187;font-size:10pt}.head .contact{font-size:8.5pt;color:#577284;line-height:1.9;max-width:68mm;text-align:left;direction:ltr;overflow-wrap:anywhere}.head .contact .fa{direction:rtl;text-align:left}
.title{display:flex;align-items:center;gap:4mm;margin:9mm 0 3mm;break-after:avoid}.title .number{background:#dceaf3;color:#245d82;border-radius:4px;min-width:9mm;height:9mm;display:grid;place-items:center;font:700 10pt Arial}.title h2{margin:0;color:#174868;font-size:12.5pt}.title:after{content:'';height:1px;background:#d7e5ec;flex:1}
.item{padding:3mm 0 3mm 4mm;border-bottom:1px solid #e6eef3;break-inside:avoid}.item:last-child{border-bottom:none}.item strong{font-size:10pt;color:#264758}.item .meta{font-size:8.5pt;color:#7890a1;margin-top:1mm}.entry-list{margin:0;padding:0 5mm 0 0}.entry-list li{padding:2.5mm 0;break-inside:avoid}.entry-list li::marker{color:#4b8aae}.chips{display:flex;flex-wrap:wrap;gap:2mm}.chip{background:#edf5f9;color:#335f79;padding:1.5mm 3mm;border-radius:4px;break-inside:avoid}.two-col{display:grid;grid-template-columns:1fr 1fr;gap:0 8mm}.two-col .item{border-bottom:1px solid #e6eef3}
.note{color:#8299a7;font-size:8pt;margin-top:8mm;border-top:1px solid #e6eef3;padding-top:3mm}
</style></head><body><div class="top"></div><main class="page">
<div class="brand">دانشگاه هرمزگان <small>پرتال اعضای هیئت علمی</small></div>
<div class="head"><div><h1>{{.Professor.Name}}</h1><div class="subtitle">{{.Professor.Rank}} · {{.Professor.Faculty}}</div>{{if .Professor.Department}}<div class="subtitle">{{.Professor.Department}}</div>{{end}}</div><div class="contact">{{if .Professor.Email}}<div>{{.Professor.Email}}</div>{{end}}{{if .Professor.Office}}<div class="fa">{{.Professor.Office}}</div>{{end}}{{if .Professor.Field}}<div class="fa">{{.Professor.Field}}</div>{{end}}</div></div>
{{if .Professor.Education}}<div class="title"><span class="number">۰۱</span><h2>سوابق تحصیلی</h2></div><div class="two-col">{{range .Professor.Education}}<div class="item"><strong>{{.Degree}} · {{.University}}</strong><div class="meta">{{if .Country}}{{.Country}}{{end}}{{if .Date}} · {{.Date}}{{end}}</div>{{if .Thesis}}<div class="meta">موضوع پایان‌نامه: {{.Thesis}}</div>{{end}}</div>{{end}}</div>{{end}}
{{if .Professor.Research.Journals}}<div class="title"><span class="number">۰۲</span><h2>مقالات مجلات</h2></div><ol class="entry-list">{{range .Professor.Research.Journals}}<li>{{.}}</li>{{end}}</ol>{{end}}
{{if .Professor.Research.Conferences}}<div class="title"><span class="number">۰۳</span><h2>همایش‌ها و کنفرانس‌ها</h2></div><ol class="entry-list">{{range .Professor.Research.Conferences}}<li>{{.}}</li>{{end}}</ol>{{end}}
{{if .Professor.Research.Books}}<div class="title"><span class="number">۰۴</span><h2>کتاب‌ها</h2></div><ol class="entry-list">{{range .Professor.Research.Books}}<li>{{.}}</li>{{end}}</ol>{{end}}
{{if .Professor.Research.Projects}}<div class="title"><span class="number">۰۵</span><h2>طرح‌های پژوهشی</h2></div><ol class="entry-list">{{range .Professor.Research.Projects}}<li>{{.}}</li>{{end}}</ol>{{end}}
{{if .Professor.Research.Patents}}<div class="title"><span class="number">۰۶</span><h2>اختراع‌ها</h2></div><ol class="entry-list">{{range .Professor.Research.Patents}}<li>{{.}}</li>{{end}}</ol>{{end}}
{{if .Professor.Teaching}}<div class="title"><span class="number">۰۷</span><h2>سوابق تدریس</h2></div>{{range .Professor.Teaching}}<div class="item"><strong>{{.Degree}}</strong><div class="meta">{{join .Subjects "، "}}</div></div>{{end}}{{end}}
{{if .Professor.Theses}}<div class="title"><span class="number">۰۸</span><h2>پایان‌نامه‌ها</h2></div>{{range .Professor.Theses}}<div class="item"><strong>{{.Title}}</strong><div class="meta">{{.Role}}{{if .Degree}} · {{.Degree}}{{end}}{{if .Year}} · {{.Year}}{{end}}</div></div>{{end}}{{end}}
{{if .Professor.Interests}}<div class="title"><span class="number">۰۹</span><h2>زمینه‌های مورد علاقه</h2></div><div class="chips">{{range .Professor.Interests}}<span class="chip">{{.}}</span>{{end}}</div>{{end}}
<div class="note">این رزومه بر اساس اطلاعات ثبت‌شده در پرتال اساتید دانشگاه هرمزگان تهیه شده است.</div>
</main></body></html>`

func RenderResumePDF(parent context.Context, professor Professor, cfg Config) ([]byte, error) {
	font, err := os.ReadFile(cfg.FontPath)
	if err != nil {
		return nil, fmt.Errorf("read resume font: %w", err)
	}
	fontCSS := template.CSS(`@font-face{font-family:IRANSans;src:url("data:font/woff2;base64,` + base64.StdEncoding.EncodeToString(font) + `") format("woff2");font-weight:400;font-display:block}`)
	tmpl, err := template.New("resume").Funcs(template.FuncMap{"join": func(items []string, separator string) string {
		result := ""
		for i, item := range items {
			if i > 0 {
				result += separator
			}
			result += item
		}
		return result
	}}).Parse(resumeHTML)
	if err != nil {
		return nil, err
	}
	var html bytes.Buffer
	if err := tmpl.Execute(&html, struct {
		Professor Professor
		FontCSS   template.CSS
	}{professor, fontCSS}); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parent, 35*time.Second)
	defer cancel()
	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Headless, chromedp.DisableGPU, chromedp.NoFirstRun)
	if cfg.ChromeBin != "" {
		options = append(options, chromedp.ExecPath(cfg.ChromeBin))
	}
	allocator, releaseAllocator := chromedp.NewExecAllocator(ctx, options...)
	defer releaseAllocator()
	browser, releaseBrowser := chromedp.NewContext(allocator)
	defer releaseBrowser()
	dataURL := "data:text/html;base64," + base64.StdEncoding.EncodeToString(html.Bytes())
	var result []byte
	var fontsReady bool
	err = chromedp.Run(browser,
		chromedp.Navigate(dataURL),
		chromedp.WaitReady("body"),
		chromedp.Poll(`document.fonts.status === "loaded"`, &fontsReady, chromedp.WithPollingTimeout(10*time.Second)),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			footer := `<div style="font-family:Arial,sans-serif;font-size:8px;color:#7890a1;width:100%;padding:0 18mm;text-align:left"><span class="pageNumber"></span> / <span class="totalPages"></span></div>`
			result, _, err = page.PrintToPDF().WithPrintBackground(true).WithPaperWidth(8.27).WithPaperHeight(11.69).WithMarginTop(0).WithMarginBottom(0.35).WithMarginLeft(0).WithMarginRight(0).WithDisplayHeaderFooter(true).WithHeaderTemplate("<span></span>").WithFooterTemplate(footer).Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, err
	}
	if !fontsReady || len(result) < 100 {
		return nil, fmt.Errorf("PDF rendering was incomplete")
	}
	return result, nil
}

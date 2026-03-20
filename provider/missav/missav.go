package missav

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"golang.org/x/text/language"
	dt "gorm.io/datatypes"

	"github.com/metatube-community/metatube-sdk-go/common/parser"
	"github.com/metatube-community/metatube-sdk-go/model"
	"github.com/metatube-community/metatube-sdk-go/provider"
	"github.com/metatube-community/metatube-sdk-go/provider/internal/scraper"
)

var (
	_ provider.MovieProvider = (*MissAV)(nil)
	_ provider.ActorProvider = (*MissAV)(nil)
	_ provider.ActorSearcher = (*MissAV)(nil)
)

const (
	Name     = "MissAV"
	Priority = 1000
)

const (
	baseURL   = "https://missav.ws/"
	actorURL  = "https://missav.ws/actresses/%s"
	searchURL = "https://missav.ws/search/%s"
)

var ErrImageNotAvailable = errors.New("image not available")

type MissAV struct {
	*scraper.Scraper
}

func New() *MissAV {
	return &MissAV{
		Scraper: scraper.NewDefaultScraper(
			Name, baseURL, Priority,
			language.Japanese,
			scraper.WithUserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 Edg/146.0.0.0"),
			scraper.WithHeaders(map[string]string{
				"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
				"Accept-Language":           "zh-CN,zh;q=0.9,en;q=0.8",
				"Cache-Control":             "no-cache",
				"Pragma":                    "no-cache",
				"Priority":                  "u=0, i",
				"Referer":                   "https://missav.ws/",
				"Sec-Ch-Ua":                 `"Chromium";v="146", "Not-A.Brand";v="24", "Microsoft Edge";v="146"`,
				"Sec-Ch-Ua-Mobile":          "?0",
				"Sec-Ch-Ua-Platform":        `"macOS"`,
				"Sec-Fetch-Dest":            "document",
				"Sec-Fetch-Mode":            "navigate",
				"Sec-Fetch-Site":            "same-origin",
				"Sec-Fetch-User":            "?1",
				"Upgrade-Insecure-Requests": "1",
			}),
		),
	}
}

func (miss *MissAV) ParseMovieIDFromURL(rawURL string) (string, error) {
	homepage, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if ext := path.Base(homepage.Path); ext != "" {
		return ext, nil
	}
	return "", err
}

func (miss *MissAV) GetMovieInfoByID(id string) (*model.MovieInfo, error) {
	return miss.GetMovieInfoByURL(fmt.Sprintf(searchURL, url.QueryEscape(id)))
}

func (miss *MissAV) GetMovieInfoByURL(rawURL string) (info *model.MovieInfo, err error) {
	id, err := miss.ParseMovieIDFromURL(rawURL)
	if err != nil {
		return
	}
	info = &model.MovieInfo{
		ID:            id,
		Number:        id,
		Provider:      miss.Name(),
		Homepage:      rawURL,
		Actors:        []string{},
		PreviewImages: []string{},
		Genres:        []string{},
	}

	c := miss.ClonedCollector()
	composedMovieURL := fmt.Sprintf(searchURL, url.QueryEscape(info.ID))

	// Age check
	c.OnHTML(`div.grid.grid-cols-2.md\:grid-cols-3.xl\:grid-cols-4.gap-5 > div:nth-child(1) > div > div.relative.aspect-w-16.aspect-h-9.rounded.overflow-hidden.shadow-lg > a:nth-child(1)`, func(e *colly.HTMLElement) {
		info.ThumbURL = e.ChildAttr("img", "data-src")
		info.CoverURL = e.ChildAttr("img", "data-src")
		info.PreviewVideoURL = e.ChildAttr("video", "data-src")
		info.PreviewImages = append(info.PreviewImages, e.Request.AbsoluteURL(e.ChildAttr("img", "data-src")))
		d := c.Clone()
		d.OnRequest(func(r *colly.Request) {
			r.Headers.Set("Referer", composedMovieURL)
		})
		d.OnResponse(func(r *colly.Response) {
			e.Response.Body = r.Body // Replace HTTP body
		})
		d.Visit(e.Request.AbsoluteURL(e.Attr("href"))) // onTime
	})

	c.OnXML(`//div[@class="sm:mx-0 mb-8 rounded-0 sm:rounded-lg"]`, func(e *colly.XMLElement) {
		info.Summary = e.ChildText(`.//div[@class="mb-1 text-secondary break-all line-clamp-none"]`)
	})

	// Preview Video
	c.OnXML(`//div[@class="aspect-w-16 aspect-h-9"]`, func(e *colly.XMLElement) {
		picUrl := e.ChildAttr("video", "data-poster")
		info.PreviewImages = append(info.PreviewImages, e.Request.AbsoluteURL(picUrl))
		info.CoverURL = picUrl
	})

	// Fields
	c.OnXML(`//div[@class="text-secondary"]`, func(e *colly.XMLElement) {
		switch e.ChildText(`.//span[1]`) {
		case "番號:":
			info.Number = strings.TrimSpace(e.ChildText(`.//span[2]`))
		case "發行日期:":
			info.ReleaseDate = parser.ParseDate(e.ChildText(`.//time`))
		case "標題:":
			info.Title = strings.TrimSpace(e.ChildText(`.//span[2]`))
		case "女優:":
			info.Actors = e.ChildTexts(`.//a`)
		case "類型:":
			info.Genres = e.ChildTexts(`.//a`)
		case "系列:":
			info.Series = strings.TrimSpace(e.ChildText(`.//a[1]`))
		case "發行商:":
			info.Director = strings.TrimSpace(e.ChildText(`.//a[1]`))
			info.Maker = strings.TrimSpace(e.ChildText(`.//a[1]`))
		case "標籤:":
			info.Label = strings.TrimSpace(e.ChildText(`.//a[1]`))
		}
	})

	defer func() {
		// Validate cover image
		if err == nil && !isValidImageURL(info.CoverURL) {
			err = ErrImageNotAvailable
		}
	}()

	err = c.Visit(composedMovieURL)
	return
}

func isValidImageURL(s string) bool {
	return len(s) > 0 && strings.Index(s, "http") == 0
}

func (miss *MissAV) GetActorInfoByID(id string) (info *model.ActorInfo, err error) {
	return miss.GetActorInfoByURL(fmt.Sprintf(actorURL, url.QueryEscape(id)))
}

func (miss *MissAV) ParseActorIDFromURL(rawURL string) (id string, err error) {
	homepage, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	if ext := path.Base(homepage.Path); ext != "" {
		id = ext
	}
	return
}

func (miss *MissAV) GetActorInfoByURL(rawURL string) (info *model.ActorInfo, err error) {
	id, err := miss.ParseActorIDFromURL(rawURL)
	if err != nil {
		return
	}

	info = &model.ActorInfo{
		ID:       id,
		Name:     id,
		Provider: miss.Name(),
		Homepage: rawURL,
		Images:   []string{},
	}

	c := miss.ClonedCollector()

	// Image (profile)
	c.OnXML(`//*/div[@class="overflow-hidden rounded-full w-24 h-24"]/img`, func(e *colly.XMLElement) {
		info.Images = append(info.Images, e.Request.AbsoluteURL(e.Attr("src")))
	})

	// Fields
	c.OnXML(`//*/div[@class="mt-2 text-sm xs:text-base text-nord9"]`, func(e *colly.XMLElement) {
		data := strings.SplitN(strings.TrimSpace(e.ChildText(`.//p[1]`)), "/", 2)
		date := strings.TrimSpace(strings.SplitN(strings.TrimSpace(e.ChildText(`.//p[2]`)), " (", 2)[0])
		if date != "" {
			info.Birthday = parseDate(date)
			info.DebutDate = parseDate(date)
		}
		if len(data) != 2 {
			return
		}

		if len(strings.TrimSpace(data[0])) > 0 {
			info.Height = parser.ParseInt(strings.TrimRight(data[0], "cm"))
		}

		if len(strings.TrimSpace(data[1])) > 0 {
			d := strings.SplitN(strings.TrimSpace(data[1]), " - ", 3)
			info.Measurements = fmt.Sprintf("B:%s / W:%s / H:%s", d[0][:len(d[0])-1], d[1], d[2])
			info.CupSize = d[0][len(d[0])-1:]
		}
	})

	err = c.Visit(info.Homepage)
	return
}

func (miss *MissAV) SearchActor(keyword string) (results []*model.ActorSearchResult, err error) {
	c := miss.ClonedCollector()
	c.OnXML(`//*/div[@class="max-w-full mb-6 text-nord4 rounded-lg"]/ul/li[1]/div`, func(e *colly.XMLElement) {
		homepage := e.Request.AbsoluteURL(
			e.ChildAttr(`.//a`, "href"))
		id, _ := miss.ParseActorIDFromURL(homepage)
		// Name
		actor := strings.TrimSpace(e.ChildText(`.//h4`))
		// Images
		var images []string
		if img := e.ChildAttr(`.//a/div/img`, "src" /* lazy loading */); img != "" {
			images = []string{e.Request.AbsoluteURL(img)}
		}

		results = append(results, &model.ActorSearchResult{
			ID:       id,
			Name:     actor,
			Images:   images,
			Provider: miss.Name(),
			Homepage: homepage,
		})
	})

	err = c.Visit(fmt.Sprintf(searchURL, url.QueryEscape(keyword)))
	return
}

func parseDate(s string) (date dt.Date) {
	defer func() {
		if !time.Time(date).IsZero() {
			return
		}
		if ss := regexp.MustCompile(`([\s\d]+)年`).
			FindStringSubmatch(s); len(ss) == 2 {
			date = dt.Date(time.Date(parser.ParseInt(ss[1]),
				2, 2, 2, 2, 2, 2, time.UTC))
		}
	}()
	return parser.ParseDate(s)
}

func init() {
	provider.Register(Name, New)
}

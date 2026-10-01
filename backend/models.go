package main

import (
	"errors"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
)

type Education struct {
	Degree     string `json:"degree"`
	University string `json:"university"`
	Country    string `json:"country,omitempty"`
	Date       string `json:"date,omitempty"`
	Thesis     string `json:"thesis,omitempty"`
}

type Teaching struct {
	Degree   string   `json:"degree"`
	Subjects []string `json:"subjects"`
}

type Research struct {
	Journals    []string `json:"journals"`
	Conferences []string `json:"conferences"`
	Books       []string `json:"books"`
	Projects    []string `json:"projects"`
	Patents     []string `json:"patents"`
}

type Thesis struct {
	Title  string `json:"title"`
	Role   string `json:"role,omitempty"`
	Degree string `json:"degree,omitempty"`
	Year   string `json:"year,omitempty"`
}

type Download struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type Professor struct {
	Slug       string      `json:"slug"`
	Name       string      `json:"name"`
	Image      string      `json:"image"`
	Rank       string      `json:"rank"`
	Faculty    string      `json:"faculty"`
	Department string      `json:"department,omitempty"`
	Field      string      `json:"field,omitempty"`
	Office     string      `json:"office,omitempty"`
	Email      string      `json:"email,omitempty"`
	Education  []Education `json:"education"`
	Teaching   []Teaching  `json:"teaching"`
	Research   Research    `json:"research"`
	Theses     []Thesis    `json:"theses"`
	Interests  []string    `json:"interests"`
	Downloads  []Download  `json:"downloads"`
	Views      int64       `json:"views,omitempty"`
	Searches   int64       `json:"searches,omitempty"`
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func (p *Professor) Normalize() {
	p.Slug = strings.TrimSpace(p.Slug)
	p.Name = strings.TrimSpace(p.Name)
	p.Rank = strings.TrimSpace(p.Rank)
	if p.Rank == "استاد" {
		p.Rank = "استاد تمام"
	}
	p.Faculty = strings.TrimSpace(p.Faculty)
	p.Email = strings.TrimSpace(p.Email)
	if p.Education == nil {
		p.Education = []Education{}
	}
	if p.Teaching == nil {
		p.Teaching = []Teaching{}
	}
	if p.Theses == nil {
		p.Theses = []Thesis{}
	}
	if p.Interests == nil {
		p.Interests = []string{}
	}
	if p.Downloads == nil {
		p.Downloads = []Download{}
	}
	if p.Research.Journals == nil {
		p.Research.Journals = []string{}
	}
	if p.Research.Conferences == nil {
		p.Research.Conferences = []string{}
	}
	if p.Research.Books == nil {
		p.Research.Books = []string{}
	}
	if p.Research.Projects == nil {
		p.Research.Projects = []string{}
	}
	if p.Research.Patents == nil {
		p.Research.Patents = []string{}
	}
}

func (p Professor) Validate() error {
	if !slugPattern.MatchString(p.Slug) || len(p.Slug) > 100 {
		return errors.New("شناسهٔ استاد نامعتبر است")
	}
	if p.Name == "" || p.Rank == "" || p.Faculty == "" {
		return errors.New("نام، مرتبه و دانشکده الزامی است")
	}
	if len(p.Name) > 160 || len(p.Faculty) > 160 {
		return errors.New("مقدار یکی از فیلدها بیش از حد طولانی است")
	}
	if p.Email != "" {
		address, err := mail.ParseAddress(p.Email)
		if err != nil || address.Address != p.Email {
			return errors.New("رایانامه نامعتبر است")
		}
	}
	if p.Image != "" && (!strings.HasPrefix(p.Image, "/images/") || strings.Contains(p.Image, "..")) {
		return errors.New("مسیر تصویر نامعتبر است")
	}
	for _, item := range p.Downloads {
		parsed, err := url.Parse(item.URL)
		if item.Title == "" || err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return errors.New("عنوان یا نشانی یکی از دانلودها نامعتبر است")
		}
	}
	return nil
}

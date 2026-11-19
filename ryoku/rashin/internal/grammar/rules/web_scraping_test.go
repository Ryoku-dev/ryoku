package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWebScraping_Scrap(t *testing.T) {
	linttest.AssertSuggestionResult(t, "webscrap", NewWebScraping(), "webscrape")
}

func TestWebScraping_Scraps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "webscraps", NewWebScraping(), "webscrapes")
}

func TestWebScraping_Scrapped(t *testing.T) {
	linttest.AssertSuggestionResult(t, "webscrapped", NewWebScraping(), "webscraped")
}

func TestWebScraping_Scrapper(t *testing.T) {
	linttest.AssertSuggestionResult(t, "webscrapper", NewWebScraping(), "webscraper")
}

func TestWebScraping_Scrappers(t *testing.T) {
	linttest.AssertSuggestionResult(t, "webscrappers", NewWebScraping(), "webscrapers")
}

func TestWebScraping_Scrapping(t *testing.T) {
	linttest.AssertSuggestionResult(t, "webscrapping", NewWebScraping(), "webscraping")
}

func TestWebScraping_ScrapOpen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web scrap", NewWebScraping(), "web scrape")
}

func TestWebScraping_ScrapsOpen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web scraps", NewWebScraping(), "web scrapes")
}

func TestWebScraping_ScrappedOpen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web scrapped", NewWebScraping(), "web scraped")
}

func TestWebScraping_ScrapperOpen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web scrapper", NewWebScraping(), "web scraper")
}

func TestWebScraping_ScrappersOpen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web scrappers", NewWebScraping(), "web scrapers")
}

func TestWebScraping_ScrappingOpen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web scrapping", NewWebScraping(), "web scraping")
}

func TestWebScraping_ScrapHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web-scrap", NewWebScraping(), "web-scrape")
}

func TestWebScraping_ScrapsHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web-scraps", NewWebScraping(), "web-scrapes")
}

func TestWebScraping_ScrappedHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web-scrapped", NewWebScraping(), "web-scraped")
}

func TestWebScraping_ScrapperHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web-scrapper", NewWebScraping(), "web-scraper")
}

func TestWebScraping_ScrappersHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web-scrappers", NewWebScraping(), "web-scrapers")
}

func TestWebScraping_ScrappingHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "web-scrapping", NewWebScraping(), "web-scraping")
}

func TestWebScraping_ScrapPage(t *testing.T) {
	linttest.AssertSuggestionResult(t, "scrap page", NewWebScraping(), "scrape page")
}

func TestWebScraping_ScrappedPages(t *testing.T) {
	linttest.AssertSuggestionResult(t, "scrapped pages", NewWebScraping(), "scraped pages")
}

func TestWebScraping_ScrapsHtml(t *testing.T) {
	linttest.AssertSuggestionResult(t, "scraps html", NewWebScraping(), "scrapes html")
}

func TestWebScraping_ScrappingWeb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "scrapping web", NewWebScraping(), "scraping web")
}

func TestWebScraping_ScrappingWebAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "SCRAPPING WEB", NewWebScraping(), "SCRAPING WEB")
}

func TestWebScraping_ScrappingWebMixedCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Scrapping Web", NewWebScraping(), "Scraping Web")
}

func TestWebScraping_WebScrapLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The goal of the project is to web scrap data from all pages of the website with capability of handling exceptions.", NewWebScraping(), "The goal of the project is to web scrape data from all pages of the website with capability of handling exceptions.")
}

func TestWebScraping_WebScrapOpenBothWordsTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Web Scrap on Jabama website to generate and analyze a dataset", NewWebScraping(), "Web Scrape on Jabama website to generate and analyze a dataset")
}

func TestWebScraping_WebScrappedOpenFirstWordTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Web scrapped an amazon page , automated the scraping, stored the data in csv file and created an email alert when the drop prices", NewWebScraping(), "Web scraped an amazon page , automated the scraping, stored the data in csv file and created an email alert when the drop prices")
}

func TestWebScraping_WebScrappedOpenBothWordsTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This project uses the data collected (Web Scrapped) from a website that list the houses for sale in Rwanda", NewWebScraping(), "This project uses the data collected (Web Scraped) from a website that list the houses for sale in Rwanda")
}

func TestWebScraping_WebScrappedHyphenatedBothWordsTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Web-Scrapped Datasets", NewWebScraping(), "Web-Scraped Datasets")
}

func TestWebScraping_WebScrapperOpenBothWordsTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Web Scrapper Built Using Golang.", NewWebScraping(), "Web Scraper Built Using Golang.")
}

func TestWebScraping_WebScrappersLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Internet bots and web scrappers that will save a lot of your time!", NewWebScraping(), "Internet bots and web scrapers that will save a lot of your time!")
}

func TestWebScraping_WebScrappersHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A Collection of web-scrappers with GUI written in Pyside6/PyQt6.", NewWebScraping(), "A Collection of web-scrapers with GUI written in Pyside6/PyQt6.")
}

func TestWebScraping_WebScrappingLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ScrapPaper is a web scrapping method to extract journal information from PubMed and Google Scholar using Python script.", NewWebScraping(), "ScrapPaper is a web scraping method to extract journal information from PubMed and Google Scholar using Python script.")
}

func TestWebScraping_WebScrappingTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Web Scrapping Examples using Beautiful Soup in Python.", NewWebScraping(), "Web Scraping Examples using Beautiful Soup in Python.")
}

func TestWebScraping_WebScrappingHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "some websites allow web-scrapping some don't.", NewWebScraping(), "some websites allow web-scraping some don't.")
}

func TestWebScraping_Webscrapped(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Example of webscrapped document : click here.", NewWebScraping(), "Example of webscraped document : click here.")
}

func TestWebScraping_Webscrapper(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A webscrapper to scrape all the words and their meanings from urban dictionary.", NewWebScraping(), "A webscraper to scrape all the words and their meanings from urban dictionary.")
}

func TestWebScraping_WebscrappersCapitalized(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A collection of Webscrappers I built using Scrapy while learning it hands on - SIdR4g/Scrapy_practice.", NewWebScraping(), "A collection of Webscrapers I built using Scrapy while learning it hands on - SIdR4g/Scrapy_practice.")
}

func TestWebScraping_WebscrappersCamelcase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Awesome-WebScrappers. Collection of powerful and efficient web scrapers built using Python and BeautifulSoup.", NewWebScraping(), "Awesome-WebScrapers. Collection of powerful and efficient web scrapers built using Python and BeautifulSoup.")
}

func TestWebScraping_Webscrapping(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Webscrapping to identify and download latest pdf documents.", NewWebScraping(), "Webscraping to identify and download latest pdf documents.")
}

func TestWebScraping_WebscrapsLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "gostapafor is a tool that webscraps and forwards html pages to other consumers", NewWebScraping(), "gostapafor is a tool that webscrapes and forwards html pages to other consumers")
}

func TestWebScraping_WebscrapsCamelcase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "WebScraps the University of California, Santa Cruz's menu and texts it to the user for Breakfast, Lunch, Dinner, and Late Night.", NewWebScraping(), "WebScrapes the University of California, Santa Cruz's menu and texts it to the user for Breakfast, Lunch, Dinner, and Late Night.")
}

func TestWebScraping_ScrapHtml(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Scrap a website's HTML, CSS and JS using this API.", NewWebScraping(), "Scrape a website's HTML, CSS and JS using this API.")
}

func TestWebScraping_ScrappedNewsArticles(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Scrapped various news article including all the details using scrapy framework.", NewWebScraping(), "Scrapped various news article including all the details using scrapy framework.")
}

func TestWebScraping_ScrappingDifferentWebsites(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This repository contains Python scripts based on Scrapping different websites.", NewWebScraping(), "This repository contains Python scripts based on Scraping different websites.")
}

func TestWebScraping_ScrappingWebContent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Scrapping web content using PHP and Python.", NewWebScraping(), "Scraping web content using PHP and Python.")
}

func TestWebScraping_ScrapsFromPages(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It requests the website and scraps news from different pages.", NewWebScraping(), "It requests the website and scrapes news from different pages.")
}

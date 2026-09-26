package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// weather.go owns the weather forecast the daemon fetches from Open-Meteo (the
// keyless public provider) and streams to QML on the "weather" state topic, so
// QML never makes an HTTP request itself. It reproduces the reference weather
// service exactly: the forecast and geocoding endpoints and their query
// parameters, the 15 minute poll, the retry ladder (3 attempts, 5 s doubling,
// 60 s on rate limit), the WMO-code to condition table, the condition to icon
// table with its day and night split, the five error strings, and the unit
// conversions (F = C*9/5+32, mph = kmh*0.621371). The forecast is always
// requested in celsius and km/h; the display unit is applied when formatting, so
// the same fetch serves either unit. Current conditions come from the hourly
// entry for the current hour (the endpoint requests no separate current block);
// hourly keeps 24 entries from that hour, daily keeps the first 7, and astronomy
// is the first daily entry's sunrise/sunset.
//
// Divergence from the reference (recorded): the reference default location is the
// null island (Coordinates 0,0). Ryoku instead falls back to a keyless IP lookup
// when no location is configured, so a fresh profile shows local weather rather
// than the Gulf of Guinea. An explicit location always wins over the fallback.

const (
	wxForecastURL  = "https://api.open-meteo.com/v1/forecast"
	wxGeocodingURL = "https://geocoding-api.open-meteo.com/v1/search"
	wxIPURL        = "https://ipwho.is/"
	wxAirURL       = "https://air-quality-api.open-meteo.com/v1/air-quality"

	wxHourlyParams = "temperature_2m,relative_humidity_2m,apparent_temperature," +
		"precipitation_probability,precipitation,weather_code,cloud_cover,pressure_msl," +
		"visibility,wind_speed_10m,wind_direction_10m,wind_gusts_10m,dew_point_2m,uv_index,is_day"
	wxDailyParams = "weather_code,temperature_2m_max,temperature_2m_min," +
		"relative_humidity_2m_mean,sunrise,sunset,uv_index_max,precipitation_sum," +
		"precipitation_probability_max,wind_speed_10m_max"
	wxAirParams = "pm10,pm2_5,ozone,european_aqi"

	wxPollInterval    = 15 * time.Minute
	wxRecoverInterval = 30 * time.Second
	wxMaxRetries      = 3
	wxRetryBase       = 5 * time.Second
	wxRateWait        = 60 * time.Second
	wxHTTPTimeout     = 10 * time.Second
	wxDailyRows       = 3
)

// Weather conditions, reproduced from the reference WeatherCondition enum. Only
// the variants Open-Meteo can produce are ever emitted; Cloudy, Mist, Windy and
// Hail come from the reference's unused key-based providers and are kept in the
// icon table for completeness but never returned by wmoCondition.
const (
	condClear        = "Clear"
	condPartlyCloudy = "PartlyCloudy"
	condCloudy       = "Cloudy"
	condOvercast     = "Overcast"
	condMist         = "Mist"
	condFog          = "Fog"
	condLightRain    = "LightRain"
	condRain         = "Rain"
	condHeavyRain    = "HeavyRain"
	condDrizzle      = "Drizzle"
	condLightSnow    = "LightSnow"
	condSnow         = "Snow"
	condHeavySnow    = "HeavySnow"
	condSleet        = "Sleet"
	condThunderstorm = "Thunderstorm"
	condWindy        = "Windy"
	condHail         = "Hail"
	condUnknown      = "Unknown"
)

// Weather error kinds and their exact display strings (reference weather service).
const (
	wxErrNetwork          = "network"
	wxErrApiKeyMissing    = "apiKeyMissing"
	wxErrLocationNotFound = "locationNotFound"
	wxErrRateLimited      = "rateLimited"
	wxErrOther            = "other"
)

func weatherErrorMessage(kind string) string {
	switch kind {
	case wxErrNetwork:
		return "Error loading weather. Check network."
	case wxErrApiKeyMissing:
		return "Error loading weather. Api key missing."
	case wxErrLocationNotFound:
		return "Error loading weather. Location not found."
	case wxErrRateLimited:
		return "Error loading weather. Too many requests."
	default:
		return "Error loading weather."
	}
}

// wmoCondition maps an Open-Meteo WMO weather code to a condition (reference
// WeatherCondition::from_wmo_code, appendix B). Codes outside the table are
// Unknown.
func wmoCondition(code int) string {
	switch code {
	case 0:
		return condClear
	case 1, 2:
		return condPartlyCloudy
	case 3:
		return condOvercast
	case 45, 48:
		return condFog
	case 51, 53, 55:
		return condDrizzle
	case 56, 57:
		return condSleet
	case 61:
		return condLightRain
	case 63:
		return condRain
	case 65:
		return condHeavyRain
	case 66, 67:
		return condSleet
	case 71:
		return condLightSnow
	case 73:
		return condSnow
	case 75:
		return condHeavySnow
	case 77:
		return condSnow
	case 80, 81, 82:
		return condRain
	case 85, 86:
		return condSnow
	case 95:
		return condThunderstorm
	case 96, 99:
		return condThunderstorm
	default:
		return condUnknown
	}
}

// conditionLabel is the short display word for a WMO code, mirroring the JS
// labelFor so the daemon can ship the hero's condition text ready to bind.
func conditionLabel(code int) string {
	switch {
	case code == 0:
		return "Clear"
	case code <= 3:
		return "Cloudy"
	case code == 45 || code == 48:
		return "Fog"
	case code >= 95:
		return "Thunder"
	case (code >= 71 && code <= 77) || code == 85 || code == 86:
		return "Snow"
	case (code >= 51 && code <= 67) || (code >= 80 && code <= 82):
		return "Rain"
	default:
		return "Cloudy"
	}
}

// weatherIcon maps a condition and day/night flag to a Ryoku weather icon token
// (reference get_weather_icon_name, appendix A). Only Clear and PartlyCloudy
// split on day/night; every other condition uses one icon for both. Daily items
// always pass isDay=true.
func weatherIcon(condition string, isDay bool) string {
	switch condition {
	case condClear:
		if isDay {
			return "wx-clear-day"
		}
		return "wx-clear-night"
	case condPartlyCloudy:
		if isDay {
			return "wx-partly-cloudy-day"
		}
		return "wx-partly-cloudy-night"
	case condCloudy:
		return "wx-cloudy"
	case condOvercast:
		return "wx-overcast"
	case condMist:
		return "wx-mist"
	case condFog:
		return "wx-fog"
	case condLightRain:
		return "wx-rain-light"
	case condRain:
		return "wx-rain"
	case condHeavyRain:
		return "wx-rain-heavy"
	case condDrizzle:
		return "wx-drizzle"
	case condLightSnow:
		return "wx-snow-light"
	case condSnow:
		return "wx-snow"
	case condHeavySnow:
		return "wx-snow-heavy"
	case condSleet:
		return "wx-sleet"
	case condThunderstorm:
		return "wx-thunderstorm"
	case condWindy:
		return "wx-windy"
	case condHail:
		return "wx-hail"
	default:
		return "wx-unknown"
	}
}

// wxImperialLocale matches the three Fahrenheit-holdout locales (US, Liberia,
// Myanmar), used to resolve the "auto" temperature unit from the environment.
var wxImperialLocale = regexp.MustCompile(`(^|[_.@-])(US|LR|MM)([_.@-]|$)`)

// resolveUnit turns the configured unit ("auto"/"celsius"/"fahrenheit") into a
// concrete one. "auto" follows the locale environment.
func resolveUnit(unit string) string {
	switch unit {
	case "celsius", "fahrenheit":
		return unit
	default:
		env := os.Getenv("LC_MEASUREMENT")
		if env == "" {
			env = os.Getenv("LANG")
		}
		if wxImperialLocale.MatchString(env) {
			return "fahrenheit"
		}
		return "celsius"
	}
}

// wxShort renders a display value as a whole number: every surface prints
// temperatures and wind at integer precision, so the strings stay tidy and
// never crowd their row.
func wxShort(v float64) string {
	return strconv.Itoa(int(math.Round(v)))
}

// fmtTemp formats a celsius value in the display unit, rounded to the degree
// (fahrenheit = c*9/5+32).
func fmtTemp(celsius float64, unit string) string {
	if unit == "fahrenheit" {
		return wxShort(celsius*9.0/5.0+32.0) + "\u00b0F"
	}
	return wxShort(celsius) + "\u00b0C"
}

// windValue formats a km/h wind speed in the display unit (mph = kmh*0.621371).
func windValue(kmh float64, unit string) string {
	if unit == "fahrenheit" {
		return wxShort(kmh * 0.621371)
	}
	return wxShort(kmh)
}

// windUnits is the suffix that follows the wind value.
func windUnits(unit string) string {
	if unit == "fahrenheit" {
		return " mph winds"
	}
	return " kmh winds"
}

// windCardinal turns a wind bearing in degrees into a 16-point compass label.
func windCardinal(deg float64) string {
	dirs := [...]string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	i := int(math.Mod(math.Round(deg/22.5), 16))
	if i < 0 {
		i += 16
	}
	return dirs[i]
}

// fmtVisibility formats a metre visibility in the display unit (km, or miles for
// the imperial holdouts).
func fmtVisibility(meters float64, unit string) string {
	if unit == "fahrenheit" {
		return strconv.Itoa(int(math.Round(meters/1609.344))) + " mi"
	}
	return strconv.Itoa(int(math.Round(meters/1000.0))) + " km"
}

// fmtPressure formats a mean-sea-level pressure in hPa.
func fmtPressure(hpa float64) string {
	return strconv.Itoa(int(math.Round(hpa))) + " hPa"
}

// fmtPrecip formats an hourly precipitation amount in the display unit (mm, or
// inches for the imperial holdouts), trimming trailing zeros.
func fmtPrecip(mm float64, unit string) string {
	if unit == "fahrenheit" {
		return strconv.FormatFloat(math.Round(mm/25.4*100)/100, 'f', -1, 64) + " in"
	}
	return strconv.FormatFloat(math.Round(mm*10)/10, 'f', -1, 64) + " mm"
}

// tempInt rounds a celsius value in the display unit to a whole number, for the
// singleton's legacy numeric fields.
func tempInt(celsius float64, unit string) int {
	if unit == "fahrenheit" {
		return int(math.Round(celsius*9.0/5.0 + 32.0))
	}
	return int(math.Round(celsius))
}

// fmtClock formats an ISO datetime or time as HH:MM (24h) or I:MM PM (12h).
func fmtClock(iso string, clock24 bool) string {
	t, ok := parseISOTime(iso)
	if !ok {
		return ""
	}
	return fmtClockTime(t, clock24)
}

// fmtClockTime formats a time value as HH:MM (24h) or I:MM PM (12h).
func fmtClockTime(t time.Time, clock24 bool) string {
	if clock24 {
		return t.Format("15:04")
	}
	return t.Format("3:04 PM")
}

// fmtHour formats an ISO datetime as the hourly-column label: %H (24h) or %I %p
// (12h), matching the reference hourly item.
func fmtHour(iso string, clock24 bool) string {
	t, ok := parseISOTime(iso)
	if !ok {
		return ""
	}
	if clock24 {
		return t.Format("15")
	}
	return t.Format("03 PM")
}

// fmtWeekday formats an ISO date as the abbreviated weekday (%a).
func fmtWeekday(iso string) string {
	t, err := time.ParseInLocation("2006-01-02", iso, time.Local)
	if err != nil {
		return ""
	}
	return t.Format("Mon")
}

func parseISOTime(iso string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02T15:04", "15:04"} {
		if t, err := time.ParseInLocation(layout, iso, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// --- fetch response shapes ---

type wxHourlyData struct {
	Time                     []string  `json:"time"`
	Temperature2m            []float64 `json:"temperature_2m"`
	RelativeHumidity2m       []float64 `json:"relative_humidity_2m"`
	ApparentTemperature      []float64 `json:"apparent_temperature"`
	PrecipitationProbability []float64 `json:"precipitation_probability"`
	WeatherCode              []float64 `json:"weather_code"`
	WindSpeed10m             []float64 `json:"wind_speed_10m"`
	UvIndex                  []float64 `json:"uv_index"`
	WindDirection10m         []float64 `json:"wind_direction_10m"`
	Precipitation            []float64 `json:"precipitation"`
	PressureMsl              []float64 `json:"pressure_msl"`
	Visibility               []float64 `json:"visibility"`
	IsDay                    []float64 `json:"is_day"`
}

type wxDailyData struct {
	Time             []string  `json:"time"`
	WeatherCode      []float64 `json:"weather_code"`
	Temperature2mMax []float64 `json:"temperature_2m_max"`
	Temperature2mMin []float64 `json:"temperature_2m_min"`
	Sunrise          []string  `json:"sunrise"`
	Sunset           []string  `json:"sunset"`
	UvIndexMax       []float64 `json:"uv_index_max"`
}

type wxForecastResponse struct {
	Hourly wxHourlyData `json:"hourly"`
	Daily  wxDailyData  `json:"daily"`
}

type wxAirData struct {
	Time        []string  `json:"time"`
	Pm10        []float64 `json:"pm10"`
	Pm25        []float64 `json:"pm2_5"`
	Ozone       []float64 `json:"ozone"`
	EuropeanAqi []float64 `json:"european_aqi"`
}

type wxAirResponse struct {
	Hourly wxAirData `json:"hourly"`
}

// wxLocation is a resolved place: coordinates plus the display name parts.
type wxLocation struct {
	city, region, country string
	lat, lon              float64
}

// locationLine builds the reference location string: "{city}, {region or
// country}", or "{lat}, {lon}" when the city is empty.
func (l wxLocation) locationLine() string {
	if l.city == "" {
		// Coordinates are an identity, not a reading: rounding them to whole
		// degrees would move the named place, so they keep their precision.
		return strconv.FormatFloat(l.lat, 'f', -1, 64) + ", " + strconv.FormatFloat(l.lon, 'f', -1, 64)
	}
	tail := l.region
	if tail == "" {
		tail = l.country
	}
	if tail == "" {
		return l.city
	}
	return l.city + ", " + tail
}

// --- published state shapes ---

type wxCurrent struct {
	Icon        string `json:"icon"`
	Code        int    `json:"code"`
	Condition   string `json:"condition"`
	IsDay       bool   `json:"isDay"`
	Temperature string `json:"temperature"`
	FeelsLike   string `json:"feelsLike"`
	Humidity    int    `json:"humidity"`
	UvIndex     int    `json:"uvIndex"`
	Wind        string `json:"wind"`
	WindUnits   string `json:"windUnits"`
	Sunrise     string `json:"sunrise"`
	Sunset      string `json:"sunset"`
	WindDir     string `json:"windDir"`
	WindDeg     int    `json:"windDeg"`
	Precip      string `json:"precip"`
	PrecipProb  int    `json:"precipProb"`
	Visibility  string `json:"visibility"`
	Pressure    string `json:"pressure"`
	Hi          int    `json:"hi"`
	Lo          int    `json:"lo"`
	High        string `json:"high"`
	Low         string `json:"low"`
	// Legacy numeric views for the sidebar consumers.
	Temp      int `json:"temp"`
	Feels     int `json:"feels"`
	WindValue int `json:"windValue"`
}

type wxHour struct {
	Time        string `json:"time"`
	Hour        string `json:"hour"`
	Icon        string `json:"icon"`
	Code        int    `json:"code"`
	IsDay       bool   `json:"isDay"`
	Temp        int    `json:"temp"`
	Temperature string `json:"temperature"`
	Uv          string `json:"uv"`
	Precip      int    `json:"precip"`
}

type wxDay struct {
	Weekday string  `json:"weekday"`
	Day     string  `json:"day"`
	Icon    string  `json:"icon"`
	Code    int     `json:"code"`
	Hi      int     `json:"hi"`
	Lo      int     `json:"lo"`
	High    string  `json:"high"`
	Low     string  `json:"low"`
	LoFrac  float64 `json:"loFrac"`
	HiFrac  float64 `json:"hiFrac"`
}

type wxMoon struct {
	Phase        int     `json:"phase"`
	Name         string  `json:"name"`
	Illumination int     `json:"illumination"`
	Fraction     float64 `json:"fraction"`
	Waxing       bool    `json:"waxing"`
}

type wxAir struct {
	Available bool    `json:"available"`
	Eaqi      int     `json:"eaqi"`
	Verdict   string  `json:"verdict"`
	Frac      float64 `json:"frac"`
	Pm25      string  `json:"pm25"`
	Pm10      string  `json:"pm10"`
	Ozone     string  `json:"ozone"`
}

type wxFrame struct {
	Status    string     `json:"status"`
	ErrorKind string     `json:"errorKind"`
	Error     string     `json:"error"`
	Location  string     `json:"location"`
	City      string     `json:"city"`
	HasData   bool       `json:"hasData"`
	Current   *wxCurrent `json:"current"`
	Hourly    []wxHour   `json:"hourly"`
	Daily     []wxDay    `json:"daily"`
	Moon      *wxMoon    `json:"moon"`
	Air       *wxAir     `json:"air"`
	UpdatedAt string     `json:"updatedAt"`
}

// wxConfig is the location and display configuration QML pushes in.
type wxConfig struct {
	query   string
	lat     float64
	lon     float64
	unit    string
	clock24 bool
}

// wxState holds the weather poller: its topic, its live config, and a wake
// channel that configure and retry use to force a fresh fetch.
type wxState struct {
	topic  *stateTopic
	client *http.Client
	sun    *sunState // the day window published to the sun-following features

	mu  sync.Mutex
	cfg wxConfig
	loc *wxLocation // last resolved place, reused while the query is unchanged

	wake chan struct{}
	quit chan struct{}
}

// hasSubscribers reports whether any client is subscribed to the topic, so a
// poll tick can be skipped when the menu is closed (reference behavior).
func (t *stateTopic) hasSubscribers() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.subs) > 0
}

// startWeather registers the weather topic and its control calls, publishes the
// initial loading frame, and starts the poll loop.
func (d *daemon) startWeather() {
	s := &wxState{
		topic:  d.registerTopic("weather"),
		client: &http.Client{Timeout: wxHTTPTimeout},
		cfg:    wxConfig{unit: "auto", clock24: true},
		sun:    d.sun,
		wake:   make(chan struct{}, 1),
		quit:   d.quit,
	}
	s.publishFrame(wxFrame{Status: "loading", Hourly: []wxHour{}, Daily: []wxDay{}})

	d.registerCall("weather.configure", func(raw json.RawMessage) (any, error) {
		var a struct {
			Location string   `json:"location"`
			Lat      *float64 `json:"lat"`
			Lon      *float64 `json:"lon"`
			Unit     string   `json:"unit"`
			Clock24  *bool    `json:"clock24"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		next := wxConfig{query: strings.TrimSpace(a.Location), unit: a.Unit, clock24: true}
		if a.Lat != nil {
			next.lat = *a.Lat
		}
		if a.Lon != nil {
			next.lon = *a.Lon
		}
		if next.unit == "" {
			next.unit = "auto"
		}
		if a.Clock24 != nil {
			next.clock24 = *a.Clock24
		}
		s.configure(next)
		return map[string]any{"ok": true}, nil
	})

	d.registerCall("weather.retry", func(json.RawMessage) (any, error) {
		s.signalWake()
		return map[string]any{"ok": true}, nil
	})

	go s.run()
}

// configure swaps the live config. A changed location query drops the cached
// resolved place so the next fetch re-geocodes; any change kicks a fresh fetch.
func (s *wxState) configure(next wxConfig) {
	s.mu.Lock()
	changed := next != s.cfg
	locChanged := next.query != s.cfg.query || next.lat != s.cfg.lat || next.lon != s.cfg.lon
	s.cfg = next
	if locChanged {
		s.loc = nil
	}
	s.mu.Unlock()
	if changed {
		s.signalWake()
	}
}

func (s *wxState) signalWake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// run is the poll loop. It fetches immediately, then on the 15-minute poll
// (skipped when nothing is subscribed) and whenever configure/retry wake it. A
// fetch that does not land a loaded frame - the common case is the daemon
// starting before the network is up at login - arms a short recovery ticker that
// re-tries every wxRecoverInterval until data lands, so the readout heals on its
// own instead of waiting out the 15-minute poll or a manual retry.
func (s *wxState) run() {
	poll := time.NewTicker(wxPollInterval)
	defer poll.Stop()
	heal := time.NewTicker(wxRecoverInterval)
	heal.Stop()
	defer heal.Stop()
	recovering := false
	arm := func(loaded bool) {
		want := !loaded
		if want == recovering {
			return
		}
		recovering = want
		if want {
			heal.Reset(wxRecoverInterval)
		} else {
			heal.Stop()
		}
	}
	arm(s.fetchOnce())
	for {
		select {
		case <-s.quit:
			return
		case <-s.wake:
			arm(s.fetchOnce())
		case <-poll.C:
			if !s.topic.hasSubscribers() {
				continue
			}
			arm(s.fetchOnce())
		case <-heal.C:
			arm(s.fetchOnce())
		}
	}
}

// fetchOnce resolves the location if needed and fetches the forecast with the
// retry ladder, publishing the loaded frame or an error frame. It reports
// whether a loaded frame was published, so run can arm its recovery ticker.
func (s *wxState) fetchOnce() bool {
	s.mu.Lock()
	cfg := s.cfg
	loc := s.loc
	s.mu.Unlock()

	unit := resolveUnit(cfg.unit)

	if loc == nil {
		resolved, kind, err := s.resolveLocation(cfg)
		if err != nil {
			s.publishError(kind)
			return false
		}
		loc = resolved
		s.mu.Lock()
		s.loc = loc
		s.mu.Unlock()
	}

	for attempt := 1; attempt <= wxMaxRetries; attempt++ {
		data, kind, err := s.fetchForecast(loc.lat, loc.lon)
		if err == nil {
			air, _ := s.fetchAir(loc.lat, loc.lon)
			s.publishFrame(buildFrame(data, air, *loc, unit, cfg.clock24))
			if len(data.Daily.Sunrise) > 0 && len(data.Daily.Sunset) > 0 {
				s.sun.observe(data.Daily.Sunrise[0], data.Daily.Sunset[0])
			}
			return true
		}
		if !wxRetryable(kind) || attempt == wxMaxRetries {
			s.publishError(kind)
			return false
		}
		delay := wxRetryBase * time.Duration(1<<(attempt-1))
		if kind == wxErrRateLimited {
			delay = wxRateWait
		}
		select {
		case <-s.quit:
			return false
		case <-s.wake:
			// A configure/retry arrived mid-backoff: restart the fetch cycle.
			s.signalWake()
			return false
		case <-time.After(delay):
		}
	}
	return false
}

func wxRetryable(kind string) bool {
	return kind == wxErrNetwork || kind == wxErrRateLimited
}

// resolveLocation turns the config into concrete coordinates. An explicit city
// query geocodes; explicit coordinates are used directly; an empty query falls
// back to a keyless IP lookup (the Ryoku divergence).
func (s *wxState) resolveLocation(cfg wxConfig) (*wxLocation, string, error) {
	if cfg.query != "" {
		return s.geocode(cfg.query)
	}
	if cfg.lat != 0 || cfg.lon != 0 {
		return &wxLocation{lat: cfg.lat, lon: cfg.lon}, "", nil
	}
	return s.ipLocate()
}

func (s *wxState) geocode(name string) (*wxLocation, string, error) {
	q := url.Values{}
	q.Set("name", name)
	q.Set("count", "1")
	var resp struct {
		Results []struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Name      string  `json:"name"`
			Admin1    string  `json:"admin1"`
			Country   string  `json:"country"`
		} `json:"results"`
	}
	if kind, err := s.getJSON(wxGeocodingURL+"?"+q.Encode(), &resp); err != nil {
		return nil, kind, err
	}
	if len(resp.Results) == 0 {
		return nil, wxErrLocationNotFound, fmt.Errorf("location not found: %s", name)
	}
	r := resp.Results[0]
	return &wxLocation{city: r.Name, region: r.Admin1, country: r.Country, lat: r.Latitude, lon: r.Longitude}, "", nil
}

// ipLocate resolves the coordinate from the client's public IP over a keyless
// HTTPS endpoint (ipwho.is). A lookup that fails or reports no success is a
// provider/connectivity condition, not a user-typed bad place, so it is returned
// as a retryable network error: the caller's recovery loop then re-tries instead
// of wedging on a permanent locationNotFound.
func (s *wxState) ipLocate() (*wxLocation, string, error) {
	var resp struct {
		Success   bool    `json:"success"`
		City      string  `json:"city"`
		Region    string  `json:"region"`
		Country   string  `json:"country"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if kind, err := s.getJSON(wxIPURL, &resp); err != nil {
		return nil, kind, err
	}
	if !resp.Success {
		return nil, wxErrNetwork, fmt.Errorf("ip lookup failed")
	}
	return &wxLocation{city: resp.City, region: resp.Region, country: resp.Country, lat: resp.Latitude, lon: resp.Longitude}, "", nil
}

// fetchForecast requests the forecast for a coordinate, always in celsius/kmh.
func (s *wxState) fetchForecast(lat, lon float64) (*wxForecastResponse, string, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', -1, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', -1, 64))
	q.Set("hourly", wxHourlyParams)
	q.Set("daily", wxDailyParams)
	q.Set("temperature_unit", "celsius")
	q.Set("wind_speed_unit", "kmh")
	q.Set("timezone", "auto")
	q.Set("forecast_days", "7")
	var resp wxForecastResponse
	if kind, err := s.getJSON(wxForecastURL+"?"+q.Encode(), &resp); err != nil {
		return nil, kind, err
	}
	return &resp, "", nil
}

// fetchAir requests the current air quality for a coordinate. It is best-effort
// (one call per forecast refresh, no retry ladder): a failure leaves the frame
// without an air block rather than failing the whole weather read.
func (s *wxState) fetchAir(lat, lon float64) (*wxAirResponse, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', -1, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', -1, 64))
	q.Set("hourly", wxAirParams)
	q.Set("timezone", "auto")
	q.Set("forecast_days", "1")
	var resp wxAirResponse
	if _, err := s.getJSON(wxAirURL+"?"+q.Encode(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// getJSON performs a GET and decodes JSON, mapping transport and status failures
// to the reference error kinds.
func (s *wxState) getJSON(rawURL string, out any) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wxHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return wxErrOther, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return wxErrNetwork, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return wxErrRateLimited, fmt.Errorf("rate limited")
	}
	if resp.StatusCode >= 500 {
		return wxErrOther, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return wxErrOther, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return wxErrOther, err
	}
	return "", nil
}

// currentHourIndex finds the entry for the current hour: the last entry whose
// time is not in the future (reference find_current_hour_index).
func currentHourIndex(times []string) int {
	now := time.Now()
	for i, t := range times {
		if parsed, ok := parseISOTime(t); ok && parsed.After(now) {
			if i == 0 {
				return 0
			}
			return i - 1
		}
	}
	return 0
}

func at(arr []float64, i int) float64 {
	if i >= 0 && i < len(arr) {
		return arr[i]
	}
	return 0
}

func clampInt(v float64, lo, hi int) int {
	n := int(math.Round(v))
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

var wxMoonNames = [...]string{
	"New Moon", "Waxing Crescent", "First Quarter", "Waxing Gibbous",
	"Full Moon", "Waning Gibbous", "Last Quarter", "Waning Crescent",
}

// moonPhase returns the 8-phase index (0 new .. 4 full .. 7 waning crescent),
// the fraction of the synodic month elapsed, and the illuminated fraction, from
// a dependency-free approximation anchored to the 2000-01-06 18:14 UTC new moon.
func moonPhase(t time.Time) (int, float64, float64) {
	const synodic = 29.53058867
	ref := time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC)
	days := t.UTC().Sub(ref).Hours() / 24.0
	phase := math.Mod(days, synodic) / synodic
	if phase < 0 {
		phase += 1
	}
	illum := (1 - math.Cos(2*math.Pi*phase)) / 2
	idx := int(math.Mod(math.Round(phase*8), 8))
	return idx, phase, illum
}

// buildMoon packages the moon phase for the frame: the 8-phase index and name,
// the percent illuminated, and the raw fraction/waxing flag the QML strip draws.
func buildMoon(t time.Time) *wxMoon {
	idx, phase, illum := moonPhase(t)
	return &wxMoon{
		Phase:        idx,
		Name:         wxMoonNames[idx],
		Illumination: int(math.Round(illum * 100)),
		Fraction:     illum,
		Waxing:       phase < 0.5,
	}
}

// aqiVerdict maps a European AQI value to its band word and a 0..1 position on
// the scale bar (0 at 0, 1 at the 100 top of the banded scale).
func aqiVerdict(eaqi float64) (string, float64) {
	frac := clampFrac(eaqi / 100.0)
	switch {
	case eaqi < 20:
		return "Good", frac
	case eaqi < 40:
		return "Fair", frac
	case eaqi < 60:
		return "Moderate", frac
	case eaqi < 80:
		return "Poor", frac
	case eaqi < 100:
		return "Very Poor", frac
	default:
		return "Extremely Poor", frac
	}
}

// aqiVal rounds a pollutant concentration to a whole number for display.
func aqiVal(v float64) string {
	return strconv.Itoa(int(math.Round(v)))
}

// clampFrac clamps a value to the 0..1 range.
func clampFrac(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// dailyRange returns the coldest low and warmest high across the first
// wxDailyRows days (the days the tab shows), so each range bar scales against
// the same span.
func dailyRange(d wxDailyData, n int, unit string) (int, int, bool) {
	rows := wxDailyRows
	if rows > n {
		rows = n
	}
	if rows <= 0 {
		return 0, 0, false
	}
	lo := tempInt(at(d.Temperature2mMin, 0), unit)
	hi := tempInt(at(d.Temperature2mMax, 0), unit)
	for i := 1; i < rows; i++ {
		if v := tempInt(at(d.Temperature2mMin, i), unit); v < lo {
			lo = v
		}
		if v := tempInt(at(d.Temperature2mMax, i), unit); v > hi {
			hi = v
		}
	}
	return lo, hi, true
}

// buildFrame turns the forecast and air-quality responses into the published
// frame, applying the display unit and clock format. Current conditions come
// from the current hour; hourly keeps 24 entries; daily keeps 7 with range-bar
// fractions over the shown days; moon phase and the update time are computed
// locally, and the air block is best-effort (nil-safe).
func buildFrame(data *wxForecastResponse, air *wxAirResponse, loc wxLocation, unit string, clock24 bool) wxFrame {
	h := data.Hourly
	idx := currentHourIndex(h.Time)
	now := time.Now()

	frame := wxFrame{
		Status:    "loaded",
		Location:  loc.locationLine(),
		City:      loc.city,
		HasData:   true,
		Hourly:    []wxHour{},
		Daily:     []wxDay{},
		UpdatedAt: fmtClockTime(now, clock24),
		Moon:      buildMoon(now),
		Air:       buildAir(air),
	}

	var sunrise, sunset string
	if len(data.Daily.Sunrise) > 0 {
		sunrise = fmtClock(data.Daily.Sunrise[0], clock24)
	}
	if len(data.Daily.Sunset) > 0 {
		sunset = fmtClock(data.Daily.Sunset[0], clock24)
	}

	// Today's high/low anchor the hero arrows.
	var todayHi, todayLo int
	var todayHigh, todayLow string
	if len(data.Daily.Temperature2mMax) > 0 {
		todayHi = tempInt(data.Daily.Temperature2mMax[0], unit)
		todayHigh = fmtTemp(data.Daily.Temperature2mMax[0], unit)
	}
	if len(data.Daily.Temperature2mMin) > 0 {
		todayLo = tempInt(data.Daily.Temperature2mMin[0], unit)
		todayLow = fmtTemp(data.Daily.Temperature2mMin[0], unit)
	}

	if idx < len(h.Time) && len(h.Time) > 0 {
		code := int(at(h.WeatherCode, idx))
		isDay := at(h.IsDay, idx) > 0.5
		cond := wmoCondition(code)
		celsius := at(h.Temperature2m, idx)
		feels := at(h.ApparentTemperature, idx)
		windKmh := at(h.WindSpeed10m, idx)
		windDeg := at(h.WindDirection10m, idx)
		frame.Current = &wxCurrent{
			Icon:        weatherIcon(cond, isDay),
			Condition:   conditionLabel(code),
			Code:        code,
			IsDay:       isDay,
			Temperature: fmtTemp(celsius, unit),
			FeelsLike:   fmtTemp(feels, unit),
			Humidity:    clampInt(at(h.RelativeHumidity2m, idx), 0, 100),
			UvIndex:     clampInt(at(h.UvIndex, idx), 0, 15),
			Wind:        windValue(windKmh, unit),
			WindUnits:   windUnits(unit),
			WindDir:     windCardinal(windDeg),
			WindDeg:     clampInt(windDeg, 0, 360),
			Precip:      fmtPrecip(at(h.Precipitation, idx), unit),
			PrecipProb:  clampInt(at(h.PrecipitationProbability, idx), 0, 100),
			Visibility:  fmtVisibility(at(h.Visibility, idx), unit),
			Pressure:    fmtPressure(at(h.PressureMsl, idx)),
			Sunrise:     sunrise,
			Sunset:      sunset,
			Hi:          todayHi,
			Lo:          todayLo,
			High:        todayHigh,
			Low:         todayLow,
			Temp:        tempInt(celsius, unit),
			Feels:       tempInt(feels, unit),
			WindValue:   int(math.Round(windKmhInUnit(windKmh, unit))),
		}
	}

	end := idx + 24
	if end > len(h.Time) {
		end = len(h.Time)
	}
	for i := idx; i < end; i++ {
		code := int(at(h.WeatherCode, i))
		isDay := at(h.IsDay, i) > 0.5
		cond := wmoCondition(code)
		celsius := at(h.Temperature2m, i)
		frame.Hourly = append(frame.Hourly, wxHour{
			Time:        fmtHour(h.Time[i], clock24),
			Hour:        hourField(h.Time[i]),
			Icon:        weatherIcon(cond, isDay),
			Code:        code,
			IsDay:       isDay,
			Temp:        tempInt(celsius, unit),
			Temperature: fmtTemp(celsius, unit),
			Uv:          strconv.Itoa(clampInt(at(h.UvIndex, i), 0, 15)) + " UV",
			Precip:      clampInt(at(h.PrecipitationProbability, i), 0, 100),
		})
	}

	d := data.Daily
	dend := 7
	if dend > len(d.Time) {
		dend = len(d.Time)
	}
	rangeLo, rangeHi, haveRange := dailyRange(d, dend, unit)
	for i := range dend {
		code := int(at(d.WeatherCode, i))
		cond := wmoCondition(code)
		hi := at(d.Temperature2mMax, i)
		lo := at(d.Temperature2mMin, i)
		hiI := tempInt(hi, unit)
		loI := tempInt(lo, unit)
		weekday := fmtWeekday(d.Time[i])
		loFrac, hiFrac := 0.0, 1.0
		if haveRange && rangeHi > rangeLo {
			span := float64(rangeHi - rangeLo)
			loFrac = clampFrac(float64(loI-rangeLo) / span)
			hiFrac = clampFrac(float64(hiI-rangeLo) / span)
		}
		frame.Daily = append(frame.Daily, wxDay{
			Weekday: weekday,
			Day:     weekday,
			Icon:    weatherIcon(cond, true),
			Code:    code,
			Hi:      hiI,
			Lo:      loI,
			High:    fmtTemp(hi, unit),
			Low:     fmtTemp(lo, unit),
			LoFrac:  loFrac,
			HiFrac:  hiFrac,
		})
	}

	return frame
}

func windKmhInUnit(kmh float64, unit string) float64 {
	if unit == "fahrenheit" {
		return kmh * 0.621371
	}
	return kmh
}

// hourField is the legacy "13" style hour label (the ISO hour digits), for the
// sidebar consumers that show a bare 24h hour.
func hourField(iso string) string {
	if len(iso) >= 13 {
		return iso[11:13]
	}
	return ""
}

// buildAir packages the current-hour air quality, or an unavailable block when
// the best-effort fetch returned nothing.
func buildAir(air *wxAirResponse) *wxAir {
	if air == nil || len(air.Hourly.Time) == 0 {
		return &wxAir{Available: false}
	}
	idx := currentHourIndex(air.Hourly.Time)
	eaqi := at(air.Hourly.EuropeanAqi, idx)
	verdict, frac := aqiVerdict(eaqi)
	return &wxAir{
		Available: true,
		Eaqi:      int(math.Round(eaqi)),
		Verdict:   verdict,
		Frac:      frac,
		Pm25:      aqiVal(at(air.Hourly.Pm25, idx)),
		Pm10:      aqiVal(at(air.Hourly.Pm10, idx)),
		Ozone:     aqiVal(at(air.Hourly.Ozone, idx)),
	}
}

func (s *wxState) publishFrame(frame wxFrame) {
	if frame.Hourly == nil {
		frame.Hourly = []wxHour{}
	}
	if frame.Daily == nil {
		frame.Daily = []wxDay{}
	}
	b, err := json.Marshal(frame)
	if err != nil {
		return
	}
	s.topic.publish(b)
}

// publishError ships an error frame, keeping the last hourly/daily so the nav
// buttons stay meaningful once data has loaded once.
func (s *wxState) publishError(kind string) {
	s.publishFrame(wxFrame{
		Status:    "error",
		ErrorKind: kind,
		Error:     weatherErrorMessage(kind),
		Hourly:    []wxHour{},
		Daily:     []wxDay{},
	})
}

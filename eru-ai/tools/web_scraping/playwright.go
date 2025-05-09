package web_scraping

import (
	"context"
	"encoding/json"

	"net/url"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"

	//"errors"
	"fmt"
	"strings"

	//"golang.org/x/net/html"
	"github.com/playwright-community/playwright-go"
)

type PlaywrightTool struct {
	tools.Tool
}

func (pwTool *PlaywrightTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &pwTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (pwTool *PlaywrightTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("PlaywrightTool Execute - Start")
	httpVersion := params["http_version"].(string)
	extarctedData := ""
	if httpVersion == "1.1" {
		extarctedData, err = extractBodyContentPlaywright_v1(ctx, params["url"].(string), params["selectors"].(string))
	}
	if httpVersion == "2.0" {
		extarctedData, err = extractBodyContentPlaywright_v2(ctx, params["url"].(string), params["selectors"].(string))
	}
	return map[string]interface{}{"content": extarctedData}, nil
}

func (pwTool *PlaywrightTool) GetSpec() tools.Tooling {
	return pwTool
}

// for future use
/* func extractBodyContent(htmlContent string, classNames string) (string, error) {
	doc, err := html.Parse(bytes.NewReader([]byte(htmlContent)))
	if err != nil {
		return "", err
	}
	var bodyContent string

	var findElementsWithClass func(*html.Node, string) []*html.Node
	findElementsWithClass = func(n *html.Node, className string) []*html.Node {
		var elements []*html.Node
		if n.Type == html.ElementNode {
			for _, attr := range n.Attr {
				if (attr.Key == "class" || attr.Key == "id") && strings.Contains(attr.Val, className) {
					elements = append(elements, n)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			elements = append(elements, findElementsWithClass(c, className)...)
		}
		return elements
	}

	var extractText func(*html.Node)
	extractText = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" && text != "\n" {
				bodyContent += text + " "
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractText(c)
		}
	}

	// Split class names and process each
	classes := strings.Split(classNames, ",")
	for _, className := range classes {
		className = strings.TrimSpace(className)
		if className == "" {
			continue
		}

		// Find elements with the specified class
		targetElements := findElementsWithClass(doc, className)
		for _, element := range targetElements {
			extractText(element)
		}
	}

	return strings.TrimSpace(bodyContent), nil
} */

func extractBodyContentPlaywright_v2(ctx context.Context, urlStr string, classNames string) (bodyContent string, err error) {
	pw, err := playwright.Run()
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	// Launch browser with additional options
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Args: []string{
			"--headless",
			"--no-sandbox",
			"--disable-setuid-sandbox",
			"--disable-gpu",
			"--no-default-browser-check",
			"--disable-blink-features=AutomationControlled",
			"--enable-http2",
			"--enable-features=NetworkService,NetworkServiceInProcess",
			"--disable-web-security",           // Allow cross-origin requests
			"--allow-running-insecure-content", // Allow mixed content
		},
		//Timeout: playwright.Float(10000),
	})
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	defer func() {
		if cerr := browser.Close(); cerr != nil {
			logs.WithContext(ctx).Error(cerr.Error())
		}
	}()
	// Create new context with viewport and user agent
	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  1920,
			Height: 1080,
		},
		IgnoreHttpsErrors: playwright.Bool(true),
		UserAgent:         playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36"),
		JavaScriptEnabled: playwright.Bool(true),
		HasTouch:          playwright.Bool(false),
		IsMobile:          playwright.Bool(false),
		ExtraHttpHeaders: map[string]string{
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
			"Accept-Encoding":           "gzip, deflate, br, zstd",
			"Accept-Language":           "en-US,en;q=0.9",
			"Connection":                "keep-alive",
			"Sec-Ch-Ua":                 `"Not A(Brand";v="8", "Chromium";v="132", "Google Chrome";v="132"`,
			"Sec-Ch-Ua-Mobile":          "?0",
			"Sec-Ch-Ua-Platform":        `"Windows"`,
			"Sec-Fetch-Dest":            "document",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Sec-Fetch-User":            "?1",
			"Cache-Control":             "no-cache",
			"Upgrade-Insecure-Requests": "1",
		},
	})
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}

	page, err := context.NewPage()
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	// Set up request handling before navigation
	//
	err = page.Route("**/*", func(route playwright.Route) {
		resourceType := route.Request().ResourceType()
		reqUrl := route.Request().URL()
		parsedURL, parseErr := url.Parse(reqUrl)
		if parseErr != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("Failed to parse URL: %v", parseErr))
			route.Abort("aborted")
			return
		}
		// Only allow document and xhr requests, abort others to save memory
		if resourceType == "document" || resourceType == "xhr" {

			// Add HTTP/2 pseudo-headers
			headers := route.Request().Headers()
			headers[":authority"] = parsedURL.Host
			headers[":method"] = route.Request().Method()
			headers[":path"] = parsedURL.Path
			if parsedURL.RawQuery != "" {
				headers[":path"] += "?" + parsedURL.RawQuery
			}
			headers[":scheme"] = parsedURL.Scheme
			route.Continue()
		} else {
			logs.WithContext(ctx).Debug(fmt.Sprintf("Blocking resource: %s (%s)", route.Request().URL(), resourceType))
			route.Abort("aborted")
		}
	})
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to set up route handler: %v", err))
		return "", err
	}
	// Add handlers to debug request headers
	page.On("request", func(request playwright.Request) {
		if request != nil {
			logs.WithContext(ctx).Info(fmt.Sprintf("Request URL: %s, Method: %s, ResourceType: %s",
				request.URL(),
				request.Method(),
				request.ResourceType(),
			))
		}
	})

	if _, err = page.Goto(urlStr, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}

	// Additional wait for page load
	/* if err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateDomcontentloaded,
		Timeout: playwright.Float(30000),
	}); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	} */
	// Additional wait for page load
	/* if err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(30000),
	}); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	} */

	selectors := strings.Split(classNames, ",")
	var contents []string

	//logs.WithContext(ctx).Info(fmt.Sprint(page.Locator("body").AllInnerTexts()))

	/* bodyC, berr := page.Locator("body").AllInnerTexts()
	logs.WithContext(ctx).Info(fmt.Sprint(bodyC))
	logs.WithContext(ctx).Info(fmt.Sprint(berr))
	*/
	for _, selector := range selectors {
		selectorStr := strings.TrimSpace(selector)
		if strings.HasPrefix(selectorStr, "#") {
			// For IDs, convert "#someId" to "[id^='someId']"
			selectorStr = fmt.Sprintf("[id^='%s']", strings.TrimPrefix(selectorStr, "#"))
		} else if strings.HasPrefix(selectorStr, ".") {
			// For classes, convert ".someClass" to "[class^='someClass']"
			selectorStr = fmt.Sprintf("[class^='%s']", strings.TrimPrefix(selectorStr, "."))
		}
		locator := page.Locator(selectorStr)

		// Get count of elements
		count, err := locator.Count()
		if err != nil {
			logs.WithContext(ctx).Debug(fmt.Sprintf("Error getting count for selector %s: %v", selector, err))
			continue
		}

		// Wait for all elements to be visible
		for i := 0; i < count; i++ {
			err = locator.Nth(i).WaitFor(playwright.LocatorWaitForOptions{
				State:   playwright.WaitForSelectorStateVisible,
				Timeout: playwright.Float(30000),
			})
			if err != nil {
				logs.WithContext(ctx).Debug(fmt.Sprintf("Element %d not visible for selector %s: %v", i, selector, err))
				continue
			}
		}

		elements, err := locator.All()
		if err != nil {
			logs.WithContext(ctx).Debug(fmt.Sprintf("No elements found for selector %s: %v", selector, err))
			continue
		}

		for _, element := range elements {
			text, err := element.TextContent()
			if err != nil {
				logs.WithContext(ctx).Debug(fmt.Sprintf("Error getting text content for selector %s: %v", selector, err))
				continue
			}
			if text != "" {
				contents = append(contents, strings.TrimSpace(text))
			}
		}
	}

	bodyContent = strings.Join(contents, " ")

	/* bodyContents, err := page.Locator("body").AllInnerTexts()
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	bodyContent = strings.Join(bodyContents, " ") */

	/* for _, entry := range entries {
		bodyContent, err = entry.Locator("body").TextContent()
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return "", err
		}

	} */

	// Create request context for API calls
	//apiContext, err := pw.Request.NewContext(playwright.APIRequestNewContextOptions{
	//	IgnoreHttpsErrors: playwright.Bool(true),
	//	ExtraHttpHeaders: map[string]string{
	//		"Content-Type": "application/json",
	//		"Accept":       "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
	//		"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	//	},
	//})
	//if err != nil {
	//	logs.WithContext(ctx).Error(err.Error())
	//	return "", err
	//}

	// Make POST request using XMLHttpRequest through page evaluation
	if err = browser.Close(); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	if err = pw.Stop(); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}

	//err = errors.New("something went wrong")
	return bodyContent, err
}

func extractBodyContentPlaywright_v1(ctx context.Context, urlStr string, classNames string) (bodyContent string, err error) {
	pw, err := playwright.Run()
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	// Launch browser with additional options
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Args: []string{
			"--headless",
			"--no-sandbox",
			"--disable-setuid-sandbox",
			"--disable-gpu",
			"--no-default-browser-check",
			"--disable-blink-features=AutomationControlled",
			"--enable-features=NetworkService,NetworkServiceInProcess",
		},
		//Timeout: playwright.Float(10000),
	})
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	defer func() {
		if cerr := browser.Close(); cerr != nil {
			logs.WithContext(ctx).Error(cerr.Error())
		}
	}()
	// Create new context with viewport and user agent
	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  1920,
			Height: 1080,
		},
		//IgnoreHttpsErrors: playwright.Bool(true),
		UserAgent:         playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36"),
		JavaScriptEnabled: playwright.Bool(true),
		HasTouch:          playwright.Bool(false),
		IsMobile:          playwright.Bool(false),
	})
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}

	page, err := context.NewPage()
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	// Set up request handling before navigation
	// /**/*
	/* err = page.Route("", func(route playwright.Route) {
		resourceType := route.Request().ResourceType()

		// Only allow document and xhr requests, abort others to save memory
		if resourceType == "document" || resourceType == "xhr" {
			logs.WithContext(ctx).Info(fmt.Sprintf("Loading resource: %s (%s)", route.Request().URL(), resourceType))
			route.Continue()
		} else {
			logs.WithContext(ctx).Debug(fmt.Sprintf("Blocking resource: %s (%s)", route.Request().URL(), resourceType))
			route.Abort("aborted")
		}
	})
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to set up route handler: %v", err))
		return "", err
	} */
	// Add handlers to debug request headers
	/* page.On("request", func(request playwright.Request) {
		if request != nil {
			logs.WithContext(ctx).Info(fmt.Sprintf("Request URL: %s, Method: %s, ResourceType: %s",
				request.URL(),
				request.Method(),
				request.ResourceType(),
			))
		}
	}) */

	if _, err = page.Goto(urlStr, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}

	// Additional wait for page load
	if err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(30000),
	}); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}

	selectors := strings.Split(classNames, ",")
	var contents []string

	/* bodyC, berr := page.Locator("body").AllInnerTexts()
	logs.WithContext(ctx).Info(fmt.Sprint(bodyC))
	logs.WithContext(ctx).Info(fmt.Sprint(berr))
	*/
	for _, selector := range selectors {
		selectorStr := strings.TrimSpace(selector)
		if strings.HasPrefix(selectorStr, "#") {
			// For IDs, convert "#someId" to "[id^='someId']"
			selectorStr = fmt.Sprintf("[id^='%s']", strings.TrimPrefix(selectorStr, "#"))
		} else if strings.HasPrefix(selectorStr, ".") {
			// For classes, convert ".someClass" to "[class^='someClass']"
			selectorStr = fmt.Sprintf("[class^='%s']", strings.TrimPrefix(selectorStr, "."))
		}
		locator := page.Locator(selectorStr)

		// Get count of elements
		count, err := locator.Count()
		if err != nil {
			logs.WithContext(ctx).Debug(fmt.Sprintf("Error getting count for selector %s: %v", selector, err))
			continue
		}

		// Wait for all elements to be visible
		for i := 0; i < count; i++ {
			err = locator.Nth(i).WaitFor(playwright.LocatorWaitForOptions{
				State:   playwright.WaitForSelectorStateVisible,
				Timeout: playwright.Float(30000),
			})
			if err != nil {
				logs.WithContext(ctx).Debug(fmt.Sprintf("Element %d not visible for selector %s: %v", i, selector, err))
				continue
			}
		}

		elements, err := locator.All()
		if err != nil {
			logs.WithContext(ctx).Debug(fmt.Sprintf("No elements found for selector %s: %v", selector, err))
			continue
		}

		for _, element := range elements {
			text, err := element.TextContent()
			if err != nil {
				logs.WithContext(ctx).Debug(fmt.Sprintf("Error getting text content for selector %s: %v", selector, err))
				continue
			}
			if text != "" {
				contents = append(contents, strings.TrimSpace(text))
			}
		}
	}

	bodyContent = strings.Join(contents, " ")
	logs.WithContext(ctx).Info(fmt.Sprint(bodyContent))
	if err = browser.Close(); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	if err = pw.Stop(); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	//err = errors.New("something went wrong")
	return bodyContent, err
}

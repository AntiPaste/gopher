package bot

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/ChimeraCoder/anaconda"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

type (
	slackChan struct {
		description string
		slackID     string
		welcome     bool
		special     bool
	}

	// Client is the HTTP client
	Client interface {
		Do(r *http.Request) (*http.Response, error)
	}

	// Logger function
	Logger func(message string, args ...interface{})

	// Bot structure
	Bot struct {
		id          string
		gerritLink  string
		name        string
		token       string
		version     string
		users       map[string]string
		client      Client
		devMode     bool
		emojiRE     *regexp.Regexp
		slackLinkRE *regexp.Regexp
		channels    map[string]slackChan
		slackBotAPI *slack.Client
		twitterAPI  *anaconda.TwitterApi
		logf        Logger
		ctx         context.Context
		dsClient    *datastore.Client
	}
)

// Init must be called before anything else in order to initialize the bot
func (b *Bot) Init(rtm *slack.RTM) error {
	b.logf("Determining bot / user IDs")
	users, err := b.slackBotAPI.GetUsers()
	if err != nil {
		return err
	}

	b.users = map[string]string{}

	for _, user := range users {
		switch user.Name {
		case "dlsniper":
			b.users["dlsniper"] = user.ID
		case "dominikh":
			b.users["dominikh"] = user.ID
		case b.name:
			if user.IsBot {
				b.id = user.ID
			}
		default:
			continue
		}
	}
	if b.id == "" {
		return errors.New("could not find bot in the list of names, check if the bot is called \"" + b.name + "\" ")
	}

	users = nil

	b.logf("Determining channels ID\n")
	publicChannels, err := b.slackBotAPI.GetChannels(true)
	if err != nil {
		return err
	}

	for _, channel := range publicChannels {
		channelName := strings.ToLower(channel.Name)
		if chn, ok := b.channels[channelName]; ok {
			chn.slackID = "#" + channel.ID
			b.channels[channelName] = chn
		}
	}

	publicChannels = nil

	b.logf("Determining groups ID\n")
	botGroups, err := b.slackBotAPI.GetGroups(true)
	for _, group := range botGroups {
		groupName := strings.ToLower(group.Name)
		if chn, ok := b.channels[groupName]; ok && b.channels[groupName].slackID == "" {
			chn.slackID = group.ID
			b.channels[groupName] = chn
		}
	}

	botGroups = nil

	b.logf("Initialized %s with ID: %s\n", b.name, b.id)

	params := slack.PostMessageParameters{AsUser: true}
	_, _, err = b.slackBotAPI.PostMessage(b.users["dlsniper"], fmt.Sprintf(`Deployed version: %s`, b.version), params)
	if err != nil {
		b.logf(`failed to deploy version: %s`, b.version)
	}

	return err
}

// TeamJoined is called when the someone joins the team
func (b *Bot) TeamJoined(event *slack.TeamJoinEvent) {
	if b.devMode {
		return
	}

	message := `Hello ` + event.User.Name + `,


Welcome to the Gophers Slack channel.
This Slack is meant to connect gophers from all over the world in a central place.
There is also a forum: https://forum.golangbridge.org, you might want to check it out as well.
We have a few rules that you can see here: http://coc.golangbridge.org.

Here's a list of a few channels you could join:
`

	for idx, val := range b.channels {
		if !val.welcome {
			continue
		}
		message += `<` + val.slackID + `|` + idx + `> -> ` + val.description + "\n"
	}

	message += `

If you want more suggestions, type "recommended channels".
There are quite a few other channels, depending on your interests or location (we have city / country wide channels).
Just click on the channel list and search for anything that crosses your mind.

To share code, you should use: https://play.golang.org/ as it makes it easy for others to help you.

If you are new to Go and want a copy of the Go In Action book, https://www.manning.com/books/go-in-action, please send an email to @wkennedy at bill@ardanlabs.com

Final thing, #general might be too chatty at times but don't be shy to ask your Go related question.


Now, enjoy the community and have fun.`

	params := slack.PostMessageParameters{AsUser: true, LinkNames: 1}
	_, _, err := b.slackBotAPI.PostMessage(event.User.ID, message, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) isBotMessage(event *slack.MessageEvent, eventText string) bool {
	return strings.HasPrefix(eventText, strings.ToLower("<@"+b.id+">")) ||
		strings.HasPrefix(eventText, strings.ToLower("<@"+b.id+">:")) ||
		strings.HasPrefix(eventText, "gopher ") ||
		strings.HasPrefix(eventText, "gopher: ") ||
		strings.HasPrefix(event.Channel, "D") // Direct message channels always starts with 'D'
}

func (b *Bot) trimBot(msg string) string {
	msg = strings.Replace(msg, strings.ToLower("<@"+b.id+">"), "", 1)
	if strings.HasPrefix(msg, "gopher:") ||
		strings.HasPrefix(msg, "gopher") {
		msg = strings.Replace(msg, strings.ToLower("gopher"), "", 1)
	}
	msg = strings.Trim(msg, " :\n")
	return msg
}

// limit access to certain functionality
func (b *Bot) specialRestrictions(restriction string, event *slack.MessageEvent) bool {
	if restriction == "golang_cls" {
		return event.Channel == b.channels["golang_cls"].slackID
	}

	return false
}

// HandleMessage will process the incoming message and
func (b *Bot) HandleMessage(event *slack.MessageEvent) {
	if event.BotID != "" || event.User == "" || event.SubType == "bot_message" {
		return
	}

	eventText := strings.Trim(strings.ToLower(event.Text), " \n\r")

	if b.devMode {
		b.logf("%#v\n", *event)
		b.logf("got message: %s\nisBotMessage: %t\n", eventText, b.isBotMessage(event, eventText))
		b.logf("channel: %s -> message: %q\n", event.Channel, b.trimBot(eventText))
		return
	}

	// All the variations of table flip seem to include this characters so... potato?
	if strings.Contains(eventText, "︵") || strings.Contains(eventText, "彡") {
		b.tableUnflip(event)
		return
	}

	if strings.Contains(eventText, "my adorable little gophers") {
		b.reactToEvent(event, "gopher")
		return
	}

	if strings.Contains(eventText, "bbq") {
		b.reactToEvent(event, "bbqgopher")
		return
	}

	if strings.Contains(eventText, "ermergerd") ||
		strings.Contains(eventText, "ermahgerd") {
		b.reactToEvent(event, "dragon")
		return
	}

	if strings.Contains(eventText, "beer me") {
		b.reactToEvent(event, "beer")
		b.reactToEvent(event, "beers")
		return
	}

	if strings.HasPrefix(eventText, "ghd/") {
		b.godoc(event, "github.com/", 4)
		return
	}

	if strings.HasPrefix(eventText, "d/") {
		b.godoc(event, "", 2)
		return
	}

	// TODO should we check for ``` or messages of a certain length?
	if !strings.Contains(eventText, "nolink") &&
		event.File != nil &&
		(event.File.Filetype == "go" || event.File.Filetype == "text") {
		b.suggestPlayground(event)
		return
	}

	if !b.isBotMessage(event, eventText) {
		return
	}

	eventText = b.trimBot(eventText)
	if b.devMode {
		b.logf("message: %q\n", eventText)
	}

	if strings.HasPrefix(eventText, "share cl") {
		b.shareCL(event, eventText)
		return
	}

	if eventText == "newbie resources" {
		b.newbieResources(event, false)
		return
	}

	if eventText == "newbie resources pvt" {
		b.newbieResources(event, true)
		return
	}

	if eventText == "recommended channels" {
		b.recommendedChannels(event)
		return
	}

	if eventText == "oss help" ||
		eventText == "oss help wanted" {
		b.ossHelp(event)
		return
	}

	if eventText == "work with forks" {
		b.goForks(event)
		return
	}

	if eventText == "block forever" {
		b.goBlockForever(event)
		return
	}

	if eventText == "http timeouts" {
		b.dealWithHTTPTimeouts(event)
		return
	}

	if eventText == "slices" {
		b.sliceUsageAndInternals(event)
		return
	}

	if eventText == "database tutorial" {
		b.goDatabaseTutorial(event)
		return
	}

	if eventText == "xkcd:standards" {
		b.xkcd(event, "https://xkcd.com/927/")
		return
	}

	if eventText == "xkcd:compiling" {
		b.xkcd(event, "https://xkcd.com/303/")
		return
	}

	if eventText == "xkcd:optimization" {
		b.xkcd(event, "https://xkcd.com/1691/")
		return
	}

	if strings.HasPrefix(eventText, "xkcd:") {
		b.xkcdAll(event)
		return
	}

	if eventText == "package layout" {
		b.packageLayout(event)
		return
	}

	if eventText == "idiomatic go" {
		b.idiomaticGo(event)
		return
	}

	if eventText == "avoid gotchas" {
		b.avoidGotchas(event)
		return
	}

	if eventText == "source code" {
		b.sourceCode(event)
		return
	}

	if strings.HasPrefix(eventText, "library for") {
		b.searchLibrary(event)
		return
	}

	if strings.Contains(eventText, "thank") ||
		eventText == "cheers" ||
		eventText == "hello" {
		b.reactToEvent(event, "gopher")
		return
	}

	if eventText == "wave" {
		b.reactToEvent(event, "wave")
		b.reactToEvent(event, "gopher")
		return
	}

	if eventText == "flip coin" ||
		eventText == "flip a coin" {
		b.replyFlipCoin(event)
		return
	}

	if eventText == "where do you live?" ||
		eventText == "stack" {
		b.replyBotLocation(event)
		return
	}

	if eventText == "version" {
		b.replyVersion(event)
		return
	}

	if eventText == "help" {
		b.help(event)
		return
	}
}

func (b *Bot) newbieResources(event *slack.MessageEvent, private bool) {
	newbieResources := slack.Attachment{
		Text: `First you should take the language tour: <http://tour.golang.org/>

Then, you should visit:
 - <https://golang.org/doc/code.html> to learn how to organize your Go workspace
 - <https://golang.org/doc/effective_go.html> be more effective at writing Go
 - <https://golang.org/ref/spec> learn more about the language itself
 - <https://golang.org/doc/#articles> a lot more reading material

There are some awesome websites as well:
 - <https://blog.gopheracademy.com> great resources for Gophers in general
 - <http://gotime.fm> awesome weekly podcast of Go awesomeness
 - <https://gobyexample.com> examples of how to do things in Go
 - <http://go-database-sql.org> how to use SQL databases in Go
 - <https://dmitri.shuralyov.com/idiomatic-go> tips on how to write more idiomatic Go code
 - <https://divan.github.io/posts/avoid_gotchas> will help you avoid gotchas in Go

There's also an exhaustive list of videos <http://gophervids.appspot.com> related to Go from various authors.

If you prefer books, you can try these:
 - <http://www.golangbootcamp.com/book>
 - <http://gopl.io/>
 - <https://www.manning.com/books/go-in-action> (if you e-mail @wkennedy at bill@ardanlabs.com you can get a free copy for being part of this Slack)

If you want to learn how to organize your Go project, make sure to read: <https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1#.ds38va3pp>.
Once you are accustomed to the language and syntax, you can read this series of articles for a walkthrough the various standard library packages: <https://medium.com/go-walkthrough>.

Finally, <https://github.com/golang/go/wiki#learning-more-about-go> will give a list of even more resources to learn Go`,
	}

	params := slack.PostMessageParameters{AsUser: true}
	params.Attachments = []slack.Attachment{newbieResources}
	whereTo := event.Channel
	if private {
		whereTo = event.User
	}
	_, _, err := b.slackBotAPI.PostMessage(whereTo, "Here are some resources you should check out if you are learning / new to Go:", params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) recommendedChannels(event *slack.MessageEvent) {
	message := slack.Attachment{}

	for idx, val := range b.channels {
		if val.special {
			continue
		}
		message.Text += `- <` + val.slackID + `|` + idx + `> -> ` + val.description + "\n"
	}

	params := slack.PostMessageParameters{AsUser: true}
	params.Attachments = []slack.Attachment{message}
	_, _, err := b.slackBotAPI.PostMessage(event.User, "Here is a list of recommended channels:", params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) suggestPlayground(event *slack.MessageEvent) {
	if event.File == nil {
		return
	}

	info, _, _, err := b.slackBotAPI.GetFileInfo(event.File.ID, 0, 0)
	if err != nil {
		b.logf("error while getting file info: %v", err)
		return
	}

	if info.Lines < 6 {
		return
	}

	req, err := http.NewRequest("GET", info.URLPrivateDownload, nil)
	req.Header.Add("User-Agent", "Gophers Slack bot")
	req.Header.Add("Authorization", "Bearer "+b.token)
	resp, err := b.client.Do(req)
	if err != nil {
		b.logf("error while fetching the file %v\n", err)
		return
	}

	file, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		b.logf("error while reading the file %v\n", err)
		return
	}

	requestBody := bytes.NewBuffer(file)

	req, err = http.NewRequest("POST", "https://play.golang.org/share", requestBody)
	if err != nil {
		b.logf("failed to get playground link: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Add("User-Agent", "Gophers Slack bot")
	req.Header.Add("Content-Length", strconv.Itoa(len(file)))

	resp, err = b.client.Do(req)
	if err != nil {
		b.logf("failed to get playground link: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b.logf("got non-200 response: %v", resp.StatusCode)
		return
	}

	linkID, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		b.logf("failed to get playground link: %v", err)
		return
	}

	params := slack.PostMessageParameters{AsUser: true}
	_, _, err = b.slackBotAPI.PostMessage(event.Channel, `The above code in playground: <https://play.golang.org/p/`+string(linkID)+`>`, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}

	_, _, err = b.slackBotAPI.PostMessage(event.User, `Hello. I've noticed you uploaded a Go file. To enable collaboration and make this easier to get help, please consider using: <https://play.golang.org>. If you wish to not link against the playground, please use "nolink" in the message. Thank you.`, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) ossHelp(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, `Here's a list of projects which could need some help from contributors like you: <https://github.com/corylanou/oss-helpwanted>`, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) goForks(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, `<http://blog.sgmansfield.com/2016/06/working-with-forks-in-go/>`, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) goBlockForever(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, `<http://blog.sgmansfield.com/2016/06/how-to-block-forever-in-go/>`, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) goDatabaseTutorial(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, `<http://go-database-sql.org/>`, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) dealWithHTTPTimeouts(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, `Here's a blog post which will help with http timeouts in Go: <https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/>`, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) sliceUsageAndInternals(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, `<https://blog.golang.org/go-slices-usage-and-internals>`, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) tableUnflip(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, `┬─┬ノ( º _ ºノ)`, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) searchLibrary(event *slack.MessageEvent) {
	searchTerm := strings.ToLower(event.Text)
	if idx := strings.Index(searchTerm, "library for"); idx != -1 {
		searchTerm = event.Text[idx+11:]
	} else if idx := strings.Index(searchTerm, "library in go for"); idx != -1 {
		searchTerm = event.Text[idx+17:]
	} else if idx := strings.Index(searchTerm, "go library for"); idx != -1 {
		searchTerm = event.Text[idx+14:]
	}

	searchTerm = b.slackLinkRE.ReplaceAllString(searchTerm, "")
	searchTerm = b.emojiRE.ReplaceAllString(searchTerm, "")

	if idx := strings.Index(searchTerm, "in go"); idx != -1 {
		searchTerm = searchTerm[:idx] + searchTerm[idx+5:]
	}

	searchTerm = strings.Trim(searchTerm, "?;., ")
	if len(searchTerm) == 0 || len(searchTerm) > 100 {
		return
	}
	searchTerm = url.QueryEscape(searchTerm)
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, `You can try to look here: <https://godoc.org/?q=`+searchTerm+`> or here <http://go-search.org/search?q=`+searchTerm+`>`, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) xkcd(event *slack.MessageEvent, imageLink string) {
	params := slack.PostMessageParameters{AsUser: true, UnfurlLinks: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, imageLink, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) xkcdAll(event *slack.MessageEvent) {
	eventText := strings.ToLower(event.Text)

	// Verify it's an integer to be nice to XKCD
	eventText = strings.TrimPrefix(eventText, "xkcd:")
	num, err := strconv.Atoi(eventText)
	if err != nil {
		// pretend we didn't hear them if they give bad data
		b.logf("Error while attempting to parse XKCD string: %v\n", err)
		return
	}

	imageLink := fmt.Sprintf("https://xkcd.com/%d/", num)

	params := slack.PostMessageParameters{AsUser: true, UnfurlLinks: true}
	_, _, err = b.slackBotAPI.PostMessage(event.Channel, imageLink, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) godoc(event *slack.MessageEvent, prefix string, position int) {
	link := event.Text[position:]
	if strings.Contains(link, " ") {
		link = link[:strings.Index(link, " ")]
	}

	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, `<https://godoc.org/`+prefix+link+`>`, params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) reactToEvent(event *slack.MessageEvent, reaction string) {
	item := slack.ItemRef{
		Channel:   event.Channel,
		Timestamp: event.Timestamp,
	}
	err := b.slackBotAPI.AddReaction(reaction, item)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) replyVersion(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.User, fmt.Sprintf("My version is: %s", b.version), params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) help(event *slack.MessageEvent) {
	message := slack.Attachment{
		Text: `- "newbie resources" -> get a list of newbie resources
- "newbie resources pvt" -> get a list of newbie resources as a private message
- "recommended channels" -> get a list of recommended channels
- "oss help" -> help the open-source community
- "work with forks" -> how to work with forks of packages
- "idiomatic go" -> learn how to write more idiomatic Go code
- "block forever" -> how to block forever
- "http timeouts" -> tutorial about dealing with timeouts and http
- "database tutorial" -> tutorial about using sql databases
- "package layout" -> learn how to structure your Go package
- "avoid gotchas" -> avoid common gotchas in Go
- "library for <name>" -> search a go package that matches <name>
- "flip a coin" -> flip a coin
- "source code" -> location of my source code
- "where do you live?" OR "stack" -> get information about where the tech stack behind @gopher
`,
	}

	params := slack.PostMessageParameters{AsUser: true}
	params.Attachments = []slack.Attachment{message}
	_, _, err := b.slackBotAPI.PostMessage(event.User, "Here's a list of supported commands", params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) replyBotLocation(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, "I'm currently living in the Clouds, powered by Google Container Engine (GKE) <https://cloud.google.com/container-engine>. I find my way to home using CircleCI <https://circleci.com> and Kubernetes (k8s) <http://kubernetes.io>. You can find my heart at: <https://github.com/gopheracademy/gopher>.", params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) replyFlipCoin(event *slack.MessageEvent) {
	buff := make([]byte, 1, 1)
	_, err := rand.Read(buff)
	if err != nil {
		b.logf("%s\n", err)
	}
	result := "heads"
	if buff[0]%2 == 0 {
		result = "tail"
	}
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err = b.slackBotAPI.PostMessage(event.Channel, fmt.Sprintf("%s", result), params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) packageLayout(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, "This article will explain how to organize your Go packages <https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1#.ds38va3pp>", params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) idiomaticGo(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, "Tips on how to write idiomatic Go code <https://dmitri.shuralyov.com/idiomatic-go>", params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) avoidGotchas(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, "Read this article if you want to understand and avoid common gotchas in Go <https://divan.github.io/posts/avoid_gotchas>", params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

func (b *Bot) sourceCode(event *slack.MessageEvent) {
	params := slack.PostMessageParameters{AsUser: true}
	_, _, err := b.slackBotAPI.PostMessage(event.Channel, "My source code is here <https://github.com/gopheracademy/gopher>", params)
	if err != nil {
		b.logf("%s\n", err)
		return
	}
}

// NewBot will create a new Slack bot
func NewBot(ctx context.Context, slackBotAPI *slack.Client, dsClient *datastore.Client, twitterAPI *anaconda.TwitterApi, httpClient Client, gerritLink, name, token, version string, devMode bool, log Logger) *Bot {
	return &Bot{
		ctx:         ctx,
		gerritLink:  gerritLink,
		name:        name,
		token:       token,
		client:      httpClient,
		version:     version,
		devMode:     devMode,
		logf:        log,
		slackBotAPI: slackBotAPI,
		dsClient:    dsClient,
		twitterAPI:  twitterAPI,

		emojiRE:     regexp.MustCompile(`:[[:alnum:]]+:`),
		slackLinkRE: regexp.MustCompile(`<((?:@u)|(?:#c))[0-9a-z]+>`),

		channels: map[string]slackChan{
			"golang-newbies": {description: "for newbie resources", welcome: true},
			"reviews":        {description: "for code reviews", welcome: true},
			"gotimefm":       {description: "for the awesome live podcast", welcome: true},
			"remotemeetup":   {description: "for remote meetup", welcome: true},
			"golang-jobs":    {description: "for jobs related to Go", welcome: true},

			"showandtell": {description: "tell the world about the thing you are working on"},
			"performance": {description: "anything and everything performance related"},
			"devops":      {description: "for devops related discussions"},
			"security":    {description: "for security related discussions"},
			"aws":         {description: "if you are interested in AWS"},
			"bbq":         {description: "Go controlling your bbq grill? Yes, we have that"},

			"general":    {description: "general channel", special: true},
			"golang_cls": {description: "https://twitter.com/golang_cls", special: true},
			"golang-cls": {description: "https://twitter.com/golang_cls", special: true},
		},
	}
}

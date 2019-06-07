package main

import (
	"encoding/json"
	"fmt"
	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
)

type SlackWebhook struct {
	slackToken            string
	gitLabUrlParserParams *GitLabUrlParserParams
}

func NewSlackWebhook(slackToken string, gitLabUrlParserParams *GitLabUrlParserParams) *SlackWebhook {
	return &SlackWebhook{slackToken: slackToken, gitLabUrlParserParams: gitLabUrlParserParams}
}

func (s *SlackWebhook) Request(body string, verifyToken bool) (string, error) {
	option := slackevents.OptionNoVerifyToken()
	if verifyToken {
		option = slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: s.slackToken})
	}
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), option)

	if err != nil {
		return "Failed: slackevents.ParseEvent", err
	}

	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(body), &r)
		if err != nil {
			return "Failed: json.Unmarshal", err
		}
		return r.Challenge, nil

	case slackevents.CallbackEvent:
		p, err := NewGitlabUrlParser(s.gitLabUrlParserParams)

		if err != nil {
			return "Failed: NewGitlabUrlParser", err
		}

		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.LinkSharedEvent:
			unfurls := map[string]slack.Attachment{}

			for _, link := range ev.Links {
				page, err := p.FetchURL(link.URL)

				if err != nil {
					return "Failed: FetchURL", err
				}

				if page == nil {
					continue
				}

				unfurls[link.URL] = slack.Attachment{
					Title:      page.Title,
					TitleLink:  link.URL,
					AuthorName: page.AuthorName,
					AuthorIcon: page.AuthorAvatarURL,
					Text:       page.Description,
					Color:      "#e24329", // c.f. https://brandcolors.net/b/gitlab
				}
			}

			if len(unfurls) == 0 {
				return "do nothing", nil
			}

			api := slack.New(s.slackToken)
			_, _, _, err := api.UnfurlMessage(ev.Channel, ev.MessageTimeStamp.String(), unfurls)

			if err != nil {
				return "Failed: UnfurlMessage", err
			}

			return "ok", nil
		}
	}

	return "", fmt.Errorf("Unknown event type: %s", eventsAPIEvent.Type)
}
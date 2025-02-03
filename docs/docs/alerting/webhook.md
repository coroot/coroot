---
sidebar_position: 7
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Webhook

In addition to built-in notification integrations like [Slack](/alerting/slack), [Microsoft Teams](/alerting/teams), 
[Pagerduty](/alerting/pagerduty), and [Opsgenie](/alerting/opsgenie), Coroot can integrate with nearly any system using Webhooks.

To configure a Webhook integration:

* Go to the **Project Settings** → **Integrations**
* Create a Webhook integration
* Paste a Webhook URL to the form
  <img alt="Coroot Webhook integration" src="/img/docs/webhook-integration.png" class="card w-800"/>
* Configure HTTP basic authentication and headers if required.
* Define templates for incidents and deployments.
* Send a test alert to check the integration.

## Template data

Notification templates are based on the [Go templating](https://golang.org/pkg/text/template) system. 
Coroot applies provided templates to the following data structures:

```go
type IncidentTemplateValues struct {
    Status string // OK, WARNING, CRITICAL
    Application struct {
       Namespace string
        Kind      string
        Name      string
    }
    Reports     []struct {
        Name    string // SLO, CPU, Memory, Net, ...
        Check   string // Availability, Latency, Memory leak, ...
        Message string // "error budget burn rate is 26x within 1 hour", "app containers have been restarted 11 times by the OOM killer", ...
    }
    URL string // backlink to the incident page
}
```

```go
type DeploymentTemplateValues struct {
    Status string // Deployed, Cancelled, Stuck
    Application struct {
        Namespace string
        Kind      string
        Name      string
    }
    Version string   // deployed application version
    Summary []string // "Availability: 87% (objective: 99%)", "CPU usage: +21% (+$37/mo)", "Memory: a memory leak detected", ...
    URL string       // backlink to the deployment page
}
```

## Examples

<Tabs queryString="example">
   <TabItem value="plain_text" label="Plain Text" default>
If you want to customize the message format, you can utilize [Go templating](https://golang.org/pkg/text/template) syntax 
to modify alert or deployment notifications based on the provided data structure. Below are sample templates you can use as examples.

Incident template:

```gotemplate
{{- if eq .Status `OK` }}
{{ .Application.Name }}@{{ .Application.Namespace }} incident resolved
{{- else }}
{{ .Status }} {{ .Application.Name }}@{{ .Application.Namespace }} is not meeting its SLOs
{{- end }}
{{- range .Reports }}
• *{{ .Name }}* / {{ .Check }}: {{ .Message }}
{{- end }}
{{ .URL }}
```

Deployment template:

```gotemplate
Deployment of {{ .Application.Name }}@{{ .Application.Namespace }}
*Status*: {{ .Status }}
*Version*: {{ .Version }}
{{- if .Summary }}
*Summary*:
{{- range .Summary }}
• {{ . }}
{{- end }}
{{- end }}
{{ .URL }}
```
   </TabItem>
   <TabItem value="json" label="JSON">
If the system you aim to integrate accepts JSON-formatted messages, you can employ the built-in json template function:

```gotemplate
{{ json . }}
```

This template will encode the incident and deployment data structures into valid JSON messages with the specified schema.

A sample of resulting incident message:

```json
{
  "status": "WARNING",
  "application": "default:Deployment:app1",
  "reports": [
    {
      "name": "SLO",
      "check": "Latency",
      "message": "error budget burn rate is 20x within 1 hour"
    },
    {
      "name": "Net",
      "check": "Network round-trip time (RTT)",
      "message": "high network latency to 2 upstream services"
    }
  ],
  "url": "http://127.0.0.1:8080/p/x0xwl4jz/app/default:Deployment:app1?incident=123ab456"
}
```

A sample of resulting deployment message:

```json
{
  "status": "Deployed",
  "application": "default:Deployment:app1",
  "version": "123ab456: app:v1.8.2",
  "summary": [
    "💔 Availability: 87% (objective: 99%)",
    "💔 CPU usage: +21% (+$37/mo) compared to the previous deployment",
    "🎉 Memory: looks like the memory leak has been fixed"
  ],
  "url": "http://127.0.0.1:8080/p/x0xwl4jz/app/default:Deployment:app1/Deployments#123ab456:123"
}
```
   </TabItem>
   <TabItem value="telegram" label="Telegram">

Telegram's [sendMessage](https://core.telegram.org/bots/api#sendmessage) API call expects JSON with two required fields: `chat_id` and `text`.

Follow the steps below to create a Telegram Bot and obtain `chat_id`:

1. Create a Telegram Bot:
   * Open Telegram and search for the `BotFather` user.
   * Start a chat with BotFather and type `/newbot`.
   * Follow the instructions to set up your new bot. You'll need to provide a name and a username for your bot.
   * Once the bot is created, BotFather will provide you with a token. Keep this token safe as you'll need it later.
2. Add Your Bot to a Chat:
   * After creating your bot, you'll want to add it to a chat. You can add it to an existing group or create a new one.
   * In Telegram, search for the chat where you want to add the bot.
   * Click on the chat name to open the chat settings.
   * Select "Add members" or "Invite to group" depending on your platform.
   * Search for your bot's username and add it to the chat.
3. Obtain Chat ID:
   * Once your bot is added to the chat, you need to obtain the chat ID.
   * Send a message to the chat where your bot is added.
   * Now, you need to fetch the chat ID. You can do this using various methods:
   * Use a Telegram bot like `@userinfobot` or `@getidsbot`. Send the command `/getid` in the chat and it will reply with the chat ID.

Sample incident template:

```gotemplate
{
  "chat_id": "-123456789",
  "text": "
{{- if eq .Status `OK` }}
[{{ .Application.Name }}@{{ .Application.Namespace }}]({{ .URL }}) incident resolved
{{- else }}
{{ .Status }} [{{ .Application.Name }}@{{ .Application.Namespace }}]({{ .URL }}) is not meeting its SLOs
{{- end }}
{{- range .Reports }}
• *{{ .Name }}* / {{ .Check }}: {{ .Message }}
{{- end }}",
  "parse_mode": "Markdown"
}
```

Sample deployment template:

```gotemplate
{
  "chat_id": "-123456789",
  "text": "
Deployment of [{{ .Application.Name }}@{{ .Application.Namespace }}]({{ .URL }})
*Status*: {{ .Status }}
*Version*: {{ .Version }}
{{- if .Summary }}
*Summary*:
{{- range .Summary }}
• {{ . }}
{{- end }}
{{- end }}",
  "parse_mode": "Markdown"
}
```
   </TabItem>
   <TabItem value="mattermost" label="Mattermost">

Mattermost's Incoming Webhook requires JSON with the `text` or `attachments` fields specified. 
Refer to the Mattermost [documentation](https://developers.mattermost.com/integrate/webhooks/incoming/) to create an incoming webhook and obtain its URL.

Sample incident template:

```gotemplate
{
  "text": "
{{- if eq .Status `OK` }}
#### [{{ .Application.Name }}@{{ .Application.Namespace }}]({{ .URL }}) incident resolved
{{- else }}
#### {{ .Status }} [{{ .Application.Name }}@{{ .Application.Namespace }}]({{ .URL }}) is not meeting its SLOs
{{- end }}
{{- range .Reports }}
• **{{ .Name }}** / {{ .Check }}: {{ .Message }}
{{- end }}"
}
```

Sample deployment template:

```gotemplate
{
  "text": "
#### Deployment of [{{ .Application.Name }}@{{ .Application.Namespace }}]({{ .URL }})
**Status**: {{ .Status }}
**Version**: {{ .Version }}
{{- if .Summary }}
**Summary**:
{{- range .Summary }}
• {{ . }}
{{- end }}
{{- end }}"
}
```
   </TabItem>
   <TabItem value="discord" label="Discord">

Discord's Webhook requires JSON with the content field specified ([doc](https://discord.com/developers/docs/resources/webhook#execute-webhook)). 
Refer to the Discord [documentation](https://support.discord.com/hc/en-us/articles/228383668-Intro-to-Webhooks) to create a webhook and obtain its URL.

Sample incident template:

```gotemplate
{
  "content": "
{{- if eq .Status `OK` }}
### [{{ .Application.Name }}@{{ .Application.Namespace }}]({{ .URL }}) incident resolved
{{- else }}
### {{ .Status }} [{{ .Application.Name }}@{{ .Application.Namespace }}]({{ .URL }}) is not meeting its SLOs
{{- end }}
{{- range .Reports }}
• **{{ .Name }}** / {{ .Check }}: {{ .Message }}
{{- end }}"
}
```
Sample deployment template:

```gotemplate
{
  "content": "
### Deployment of [{{ .Application.Name }}@{{ .Application.Namespace }}]({{ .URL }})
**Status**: {{ .Status }}
**Version**: {{ .Version }}
{{- if .Summary }}
**Summary**:
{{- range .Summary }}
• {{ . }}
{{- end }}
{{- end }}"
}
```

   </TabItem>
</Tabs>







---
sidebar_position: 2
---

# Configuration

:::info
AI-powered Root Cause Analysis is available only in Coroot Enterprise (from $1 per CPU core/month). [Start](https://coroot.com/account) your free trial today.
:::

Coroot Enterprise Edition supports integration with multiple AI model providers:

* Anthropic (Claude Opus 4.6) – recommended, as it delivered the best results based on our tests
* OpenAI (GPT-5.2)
* Any OpenAI-compatible API, such as DeepSeek or Google Gemini

To set up an integration, go to **Project Settings** → **AI**.
This is a global setting that applies to all projects and requires the `settings.edit` permission.

## Anthropic

To integrate with Anthropic models, simply provide your API key.
Make sure your Coroot Enterprise instance can reach `api.anthropic.com:443`.

<img alt="Anthropic" src="/img/docs/ai/anthropic.png" class="w-1200"/>

## OpenAI

To integrate with OpenAI models, provide your API key.
Make sure your Coroot Enterprise instance can connect to `api.openai.com:443`

<img alt="OpenAI" src="/img/docs/ai/openai.png" class="w-1200"/>

## OpenAI-compatible APIs

Coroot also supports any API that is compatible with OpenAI.
We’ve tested integrations with providers like Google Gemini and DeepSeek.

To configure this, provide your API key, set the base URL of your provider, and specify the model name you want to use.
Make sure your Coroot Enterprise instance can connect to the specified base URL.

<img alt="OpenAI-compatible API" src="/img/docs/ai/openai_compatible.png" class="w-1200"/>

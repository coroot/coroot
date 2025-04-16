---
sidebar_position: 4
---

# Microsoft Teams

## Configure Microsoft Teams

To configure an **Incoming webhook with Workflows** in your Microsoft Teams:
* Choose a channel (or create a new one)
* Click **...** next to the channel and select **Workflows**
  <img alt="MS Teams integration step 1" src="/img/docs/teams-integration-step1.png" class="card w-800"/>
* Choose the **Post to a channel when a webhook request is received** workflow template
  <img alt="MS Teams integration step 2" src="/img/docs/teams-integration-step2.png" class="card w-600"/>
* Provide a name (e.g., _Coroot_) and click **Next**
  <img alt="MS Teams integration step 3" src="/img/docs/teams-integration-step3.png" class="card w-800"/>
* Click **Add workflow**
  <img alt="MS Teams integration step 4" src="/img/docs/teams-integration-step4.png" class="card w-800"/>
* Copy the workflow URL and click **Done**
  <img alt="MS Teams integration step 5" src="/img/docs/teams-integration-step5.png" class="card w-800"/>

:::info
If you ever need to copy the workflow URL again, you’ll be able to find it by opening the Workflows app within Teams, 
selecting the workflow that was created, selecting **Edit**, and expanding the trigger **When a Teams webhook request is received**.
:::
<img alt="MS Teams integration step 5" src="/img/docs/teams-integration-url.png" class="card w-1200"/>
For more information, refer to the [Webhooks with Workflows for Microsoft Teams](https://support.microsoft.com/en-us/office/create-incoming-webhooks-with-workflows-for-microsoft-teams-8ae491c7-0394-4861-ba59-055e33f75498) documentation.

## Configure Coroot

* Go to the **Project Settings**  → **Integrations**
* Create an MS Teams integration
* Paste the workflow URL to the form
  <img alt="MS Teams integration" src="/img/docs/teams-integration.png" class="card w-800"/>

* You can also send a test alert to check the integration
  <img alt="MS Teams Test Alert" src="/img/docs/teams-integration-test.png" class="card w-800"/>

---
sidebar_position: 3
---

# Slack

If you want to receive alerts to your Slack channel, you’ll need to create a Slack App and make it available to Coroot.

To configure a slack integration go to the **Project Settings**  → **Integrations**.

Click on **Create Slack app**. Coroot will open a new browser tab and send you over to the Slack website to create the Slack app. Select your Slack workspace.

When you click on Create Slack app, Coroot will pass along the app manifest, which Slack will use to set up your app.

:::info
You may get a warning that says: **This app is created from a 3rd party manifes**t. 
This warning is expected (Coroot is the third party here). You can click on Configure to see the app manifest Coroot sent along in the URL. 
The manifest just take cares of some settings for your app and helps speed things along.
:::

On the Slack site for your newly created app, in the **Settings** > **Basic Information** tab, under **Install your app**, click on **Install to workspace**.

<img alt="Creating a Slack app" src="/img/docs/slack-integration-step1.png" class="card w-800"/>

On the next screen, click **Allow** to give Coroot access to your Slack workspace.

On the same page you can customize the app icon (you can use the [Coroot logo](https://coroot.com/static/img/coroot_512.png))

<img alt="Customize Slack App" src="/img/docs/slack-integration-step2.png" class="card w-600"/>

Then go to **OAuth and Permissions** and copy the **Bot User OAuth Token**.

<img alt="Slack Bot Token" src="/img/docs/slack-integration-step3.png" class="card w-800"/>

On the Coroot side:
* Go to the **Project settings**  → **Integrations**
* Create a Slack integration
* Paste the token to the form
  <img alt="Coroot Slack Integration" src="/img/docs/slack-integration.png" class="card w-800"/>

* Coroot can send alerts into any public channel in your Slack workspace. Enter that channel’s name in the **Slack channel Name** field
* You can also send a test alert to check the integration
  <img alt="Coroot Slack Test Alert" src="/img/docs/slack-integration-test.png" class="card w-600"/>






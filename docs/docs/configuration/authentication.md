---
sidebar_position: 2
---

# Authentication

After installation, Coroot will prompt you to set a password for the admin user:

<img alt="Setting Admin Password" src="/img/docs/admin_password.png" class="card w-1200"/>

To prevent someone else from setting the admin password before you, you can specify the initial password using the 
`--auth-bootstrap-admin-password` CLI argument or the `AUTH_BOOTSTRAP_ADMIN_PASSWORD` environment variable. 
This initial password can be changed later through the UI.

## Anonymous mode

To disable authentication, use the `--auth-anonymous-role` CLI argument or the `AUTH_ANONYMOUS_ROLE` environment variable, 
setting it to one of the following roles: `Admin`, `Editor`, or `Viewer`.

## Reset admin password

To reset admin password, use the following command:

```bash
$ coroot set-admin-password
Enter new password:
Confirm new password:
Admin password set successfully.
```

## Manage users

To manage Coroot users, go to the **Project Settings**, click on **Organization**:

<img alt="Manage Users" src="/img/docs/users.png" class="card w-1200"/>

To add a new user, click "Add user", fill out the form, and select a role.

<img alt="Add user" src="/img/docs/add_user.png"  class="card w-600"/>

The Coroot Community Edition includes three predefined roles: `Admin`, `Editor`, and `Viewer`. 
The Enterprise Edition allows you to create custom roles with granular permissions.

## Single Sign-On (SSO)

:::info
Single Sign-On is available only in Coroot Enterprise (from $1 per CPU core/month). [Start](https://coroot.com/account) your free trial today.
:::

Single Sign-On (SSO) feature streamlines user authentication by allowing team members to access the Coroot platform using
a single set of credentials linked to an identity provider, such as Google Workspace, Okta, or other SSO solutions.
With SSO, users no longer need to manage separate passwords for Coroot, enhancing both security and user experience.

Coroot supports two SSO protocols:
* **SAML 2.0** - Coroot acts as the service provider (SP) and communicates with your identity provider (IdP) using SAML assertions.
* **OIDC (OpenID Connect)** - A protocol built on OAuth 2.0, commonly used by Google, Azure AD, Okta, and other providers.

### Setup SAML with Okta

* Log in to the [Okta portal](https://login.okta.com/).
* Go to the Admin Console in your Okta organization.
* Navigate to **Applications** > **Applications**.
* Click **Create App Integration**.
* Select **SAML 2.0** as the Sign-in method.
* Click Next.
* On the General Settings tab, enter a name for your Coroot integration. You can also upload the [logo](https://coroot.com/static/img/coroot_512.png).
  <img alt="Okta app" src="/img/docs/saml_okta_app.png" class="card w-600"/>
* On the Configure SAML tab:
  * For both Single sign on URL and Audience URI (SP Entity ID) fields use the https://COROOT_ADDRESS/sso/saml URL.  
    <img alt="SAML Okta params" src="/img/docs/saml_okta_params.png"  class="card w-600"/>
  * In the Attribute Statements section, configure `Email`, `FirstName`, and `LastName` attributes.
    <img alt="Okta SAML attributes" src="/img/docs/saml_okta_attributes.png" class="card w-600"/>
* Click Next.
* On the final Feedback tab, fill out the form and then click Finish.
* Download **Identity Provider Metadata XML** using the `Metadata URL`:
  <img alt="Okta SAML metadata" src="/img/docs/saml_okta_metadata.png" class="card w-600"/>
* [Configure and enable](#configure-saml-for-coroot) SAML authentication for Coroot.

### Setup SAML with Keycloak

* Log in to Keycloak as an administrator.
* Select **Clients**, then click **Create client**.
  <img alt="Keycloak client general settings" src="/img/docs/saml_keycloak_client_general_settings.png" class="card w-600"/>
* Click **Next** and configure the **Home URL** and **Valid redirect URIs** fields.
  <img alt="Keycloak client login settings" src="/img/docs/saml_keycloak_client_login_settings.png" class="card w-600"/>
* **Save** the client.
* Under the **Keys** tab, set **Client signature required** to **Off**.
  <img alt="Keycloak client keys settings" src="/img/docs/saml_keycloak_client_keys_settings.png" class="card w-600"/>
* Navigate to the **Client scopes** tab and click **http://&lt;COROOT ADDRESS&gt;/sso/saml-dedicated**.
  <img alt="Keycloak client scopes" src="/img/docs/saml_keycloak_client_scopes.png" class="card w-600"/>
* Click **Add predefined mapper**, select the **X500 email**, **X500 givenName**, and **X500 surname** attributes, and click **Add**.
  <img alt="Keycloak client mappers" src="/img/docs/saml_keycloak_client_mappers.png" class="card w-600"/>
* Configure attributes mapping.
  :::info
  Coroot expects to receive the following attributes: <b>Email</b>, <b>FirstName</b>, and <b>LastName</b>
  :::
  <img alt="Keycloak client mappers" src="/img/docs/saml_keycloak_client_attributes.png" class="card w-600"/>
  * Click **X500 email** and set **SAML Attribute Name** to _Email_, and **SAML Attribute NameFormat** to _Basic_.
    <img alt="Keycloak client mappers Email" src="/img/docs/saml_keycloak_client_attributes_email.png" class="card w-600"/>
  * Click **X500 givenName** and set **SAML Attribute Name** to _FirstName_, and **SAML Attribute NameFormat** to _Basic_.
    <img alt="Keycloak client mappers Email" src="/img/docs/saml_keycloak_client_attributes_firstname.png" class="card w-600"/>
  * Click **X500 surname** and set **SAML Attribute Name** to _LastName_, and **SAML Attribute NameFormat** to _Basic_.
    <img alt="Keycloak client mappers Email" src="/img/docs/saml_keycloak_client_attributes_lastname.png" class="card w-600"/>
* Within you realm, select **Realm settings** and download **SAML 2.0 Identity Provider Metadata**
  <img alt="Keycloak SAML metadata" src="/img/docs/saml_keycloak_metadata.png" class="card w-600"/>
* [Configure and enable](#configure-saml-for-coroot) SAML authentication for Coroot.

### Configure SAML for Coroot

* Navigate to the **Project Settings** > **Organization** > **Single Sign-On (SAML)** section.
  <img alt="SSO" src="/img/docs/saml_upload_metadata.png" class="card w-800"/>
  
* Use the **Upload Identity Provider Metadata XML** button to upload the IDP metadata file that was previously downloaded.

* Click Save and Enable.
  <img alt="SSO Enabled" src="/img/docs/saml_enabled.png"  class="card w-800"/>

* Once Single Sign-On is enabled, users can click the "Login with SSO" button on the login page to authenticate through the Identity Provider.

Each team member authenticated through the Identity Provider will be displayed in the Users list in Coroot, allowing you to manually change their roles.

### Setup OIDC with Google Workspace

* Go to the [Google Cloud Console](https://console.cloud.google.com/).
* Select your project or create a new one.
* Navigate to **APIs & Services** > **Credentials**.
* Click **Create Credentials** > **OAuth client ID**.
* Select **Web application** as the application type.
* Enter a name for your OAuth client (e.g., "Coroot SSO").
* Under **Authorized redirect URIs**, add: `https://COROOT_ADDRESS/sso/oidc`
  <img alt="Google OAuth redirect URI" src="/img/docs/google_oidc.png" class="card w-600"/>
* Click **Create**.
* Copy the **Client ID** and **Client Secret**.
* [Configure and enable](#configure-oidc-for-coroot) OIDC authentication for Coroot using:
  * Issuer URL: `https://accounts.google.com`
  * Client ID and Client Secret from the previous step

### Setup OIDC with Azure AD (Entra ID)

* Log in to the [Azure Portal](https://portal.azure.com/).
* Navigate to **Microsoft Entra ID** (formerly Azure Active Directory).
* Go to **App registrations** > **New registration**.
* Enter a name for your application (e.g., "Coroot SSO").
* Under **Redirect URI**, select **Web** and enter: `https://COROOT_ADDRESS/sso/oidc`
* Click **Register**.
* On the application overview page, copy the **Application (client) ID** and **Directory (tenant) ID**.
* Navigate to **Certificates & secrets** > **New client secret**.
* Add a description and select an expiration period.
* Copy the **Value** of the new secret (this is your Client Secret).
* [Configure and enable](#configure-oidc-for-coroot) OIDC authentication for Coroot using:
  * Issuer URL: `https://login.microsoftonline.com/{tenant-id}/v2.0` (replace `{tenant-id}` with your Directory ID)
  * Client ID and Client Secret from the previous steps

### Configure OIDC for Coroot

* Navigate to the **Project Settings** > **Organization** > **Single Sign-On** section.
* Select **OIDC** as the provider.
  <img alt="OIDC Configuration" src="/img/docs/oidc_form.png" class="card w-800"/>

* Enter the following:
  * **Issuer URL**: The URL of your identity provider (e.g., `https://accounts.google.com`)
  * **Client ID**: The client ID from your identity provider
  * **Client Secret**: The client secret from your identity provider
  * **Default Role**: The role assigned to new users authenticated through SSO

* Copy the **Redirect URI** displayed and ensure it matches the redirect URI configured in your identity provider.

* Click **Save and Enable**.

* Once OIDC is enabled, users can click the "Login with SSO" button on the login page to authenticate.

<img alt="Login with SSO" src="/img/docs/login_with_sso.png" class="card w-800"/>

:::info
Coroot expects to receive the **email**, **given_name**, and **family_name** claims from the ID token.
Most OIDC providers include these claims by default when the `openid`, `profile`, and `email` scopes are requested.
:::

### Troubleshooting

Use http://&lt;COROOT_ADDRESS&gt;/login page and the **admin** user credentials to log in to your Coroot instance if you encounter any issues with SSO.


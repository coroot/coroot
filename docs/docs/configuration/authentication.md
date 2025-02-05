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

Coroot's Single Sign-On (SSO) uses the SAML protocol, where Coroot acts as the service provider (SP). 
SAML allows users to log in through an identity provider (IdP) and access Coroot without needing separate credentials. 
This makes the login process easier and more secure by centralizing authentication through the IdP.

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
  <img alt="Add user" src="/img/docs/saml_okta_metadata.png" class="card w-600"/>

### Configure SAML authentication in Coroot

* Navigate to the **Project Settings** > **Organization** > **Single Sign-On (SAML)** section.
  <img alt="SSO" src="/img/docs/saml_upload_metadata.png" class="card w-800"/>
  
* Use the **Upload Identity Provider Metadata XML** button to upload the IDP metadata file that was previously downloaded.

* Click Save and Enable.
  <img alt="SSO Enabled" src="/img/docs/saml_enabled.png"  class="card w-800"/>

* Once Single Sign-On is enabled, Coroot will redirect your team members to the Identity Provider for authentication.

Each team member authenticated through the Identity Provider will be displayed in the Users list in Coroot, allowing you to manually change their roles.

Use http://COROOT_ADDRESS/login page and the admin user credentials to log in to your Coroot instance if you encounter any issues with SSO.


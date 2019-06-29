<template>
  <div class="ui row">
    <!-- #Header -->
    <div id="pageHeader" class="ui grid">
      <div class="seven wide column">
        <h2>Settings</h2>
      </div>
      <div class="nine wide column page-actions">

      </div>
    </div><!-- #Header -->

    <!-- #Tabs -->
    <div class="ui pointing secondary menu">
      <a class="active item" data-tab="app">
        <i class="wrench horizontal icon let"></i> General
      </a>
      <a class="item" data-tab="auth">
        <i class="lock icon left"></i> OAuth2
      </a>
      <a class="item" data-tab="mail">
        <i class="envelope icon left"></i> E-mail
      </a>
      <a class="item" data-tab="vpn">
        <i class="cloud icon left"></i> OpenVPN
      </a>
    </div><!-- #Tabs -->

    <!-- #ApplicationTab -->
    <div class="ui active tab form" data-tab="app" style="padding: 1em;">
      <table class="ui very basic table">
        <tbody>
        <!-- #Name -->
        <form-table-row>
          <template slot="label">
            Organization Name
          </template>
          <template slot="help">
            The name of your organization that will be displayed on the header of the frontend UI and used when sending
            application e-mails.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.app_organization">
            <p class="form-error">
              {{ errors.app_organization | error }}
            </p>
          </template>
        </form-table-row><!-- #Name -->

        <!-- #WebHostname -->
        <form-table-row>
          <template slot="label">
            Hostname
          </template>
          <template slot="help">
            The hostname or IP address that will be used by users when accessing the frontend web
            application and generating application links.
          </template>
          <template slot="input">
            <input type="text" v-model="settings.app_hostname">
            <p class="form-error">
              {{ errors.app_hostname | error }}
            </p>
          </template>
        </form-table-row><!-- #WebHostname -->

        <!-- #HttpPort -->
        <form-table-row>
          <template slot="label">
            HTTP Port
          </template>
          <template slot="help">
            The TCP port number for which the web application will listen for incoming HTTP traffic.
            Firewall rules will automatically be added for this port.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.app_http_port">
            <p class="form-error">
              {{ errors.app_http_port | error }}
            </p>
          </template>
        </form-table-row><!-- #HttpPort -->

        <!-- #HttpsPort -->
        <form-table-row>
          <template slot="label">
            HTTPs Port
          </template>
          <template slot="help">
            The TCP port number for which the web application will listen for incoming HTTPs traffic.
            Firewall rules will automatically be added for this port.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.app_https_port">
            <p class="form-error">
              {{ errors.app_https_port | error }}
            </p>
          </template>
        </form-table-row><!-- #HttpsPort -->

        <!-- #SSLCertificate -->
        <form-table-row>
          <template slot="label">
            SSL Certificate
          </template>
          <template slot="help">
            The SSL certificate that will be used to encrypt traffic for the web application.
          </template>
          <template slot="input">
            <textarea type="text" class="ui textarea" v-model="settings.app_ssl_crt"></textarea>
            <p class="form-error">
              {{ errors.app_ssl_crt | error }}
            </p>
          </template>
        </form-table-row><!-- #SSLCertificate -->

        <!-- #SSLPrivateKey -->
        <form-table-row>
          <template slot="label">
            SSL Private Key
          </template>
          <template slot="help">
            The SSL private key that will be used to encrypt traffic for the web application.
          </template>
          <template slot="input">
            <textarea type="text" class="ui textarea" v-model="settings.app_ssl_key"></textarea>
            <p class="form-error">
              {{ errors.app_ssl_key | error }}
            </p>
          </template>
        </form-table-row><!-- #SSLPrivateKey -->
        </tbody>
      </table>
    </div><!-- #SSLPrivateKey -->

    <!-- #OAuth2 -->
    <div class="ui tab form" data-tab="auth" style="padding: 1em;">
      <table class="ui very basic table">
        <tbody>
        <!-- #OAuth2Provider -->
        <form-table-row>
          <template slot="label">
            OAuth2 Provider
          </template>
          <template slot="help">
            The OAuth2 provider that will be used when authenticating users.
          </template>
          <template slot="input">
            <select id="oauth2Provider" class="ui dropdown" v-model="settings.oauth2_provider">
              <option value="none">
                Disabled
              </option>
              <option value="google">
                Google
              </option>
            </select>
            <p class="form-error">
              {{ errors.oauth2_provider | error }}
            </p>
          </template>
        </form-table-row><!-- #OAuth2Provider -->

        <!-- #OAuth2ClientID -->
        <form-table-row v-show="settings.oauth2_provider !== 'none'">
          <template slot="label">
            OAuth2 Client ID
          </template>
          <template slot="help">
            The OAuth2 client ID from a supported OAuth2 provider.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.oauth2_client_id">
            <p class="form-error">
              {{ errors.oauth2_client_id | error }}
            </p>
          </template>
        </form-table-row><!-- #OAuth2ClientID -->

        <!-- #OAuth2ClientSecret -->
        <form-table-row v-show="settings.oauth2_provider !== 'none'">
          <template slot="label">
            OAuth2 Client Secret
          </template>
          <template slot="help">
            The OAuth2 client secret from a supported OAuth2 provider.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.oauth2_client_secret">
            <p class="form-error">
              {{ errors.oauth2_client_secret | error }}
            </p>
          </template>
        </form-table-row><!-- #OAuth2ClientSecret -->
        </tbody>
      </table>
    </div><!-- #OAuth2 -->

    <!-- #E-mail -->
    <div class="ui tab form" data-tab="mail" style="padding: 1em;">
      <table class="ui very basic table">
        <tbody>
        <!-- #SmtpServer -->
        <form-table-row>
          <template slot="label">
            Server
          </template>
          <template slot="help">
            The hostname or IP address of the SMTP server that will be used to
            send application e-mails.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.smtp_host">
            <p class="form-error">
              {{ errors.smtp_host | error }}
            </p>
          </template>
        </form-table-row><!-- #SmtpServer -->

        <!-- #SmtpPort -->
        <form-table-row>
          <template slot="label">
            Port
          </template>
          <template slot="help">
            The port number used by the SMTP server to accept incoming request. This
            will usually be 25, 465, or 587.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.smtp_port">
            <p class="form-error">
              {{ errors.smtp_port | error }}
            </p>
          </template>
        </form-table-row><!-- #SmtpPort -->

        <!-- #SmtpUsername -->
        <form-table-row>
          <template slot="label">
            Username
          </template>
          <template slot="help">
            The username for the account that will be used to send all
            application e-mails.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.smtp_username">
            <p class="form-error">
              {{ errors.smtp_username | error }}
            </p>
          </template>
        </form-table-row><!-- #SmtpUsername -->

        <!-- #SmtpPassword -->
        <form-table-row>
          <template slot="label">
            Password
          </template>
          <template slot="help">
            The password for the account that will be used to send all
            application e-mails.
          </template>
          <template slot="input">
            <input type="password" class="input" v-model="settings.smtp_password">
            <p class="form-error">
              {{ errors.smtp_password | error }}
            </p>
          </template>
        </form-table-row><!-- #SmtpPassword -->

        <!-- #SmtpReplyAddress -->
        <form-table-row>
          <template slot="label">
            Sender Name
          </template>
          <template slot="help">
            The name that is displayed to the user in the sender field. This should
            be a user-friendly name that describes your organization.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.smtp_reply_address">
            <p class="form-error">
              {{ errors.smtp_reply_address | error }}
            </p>
          </template>
        </form-table-row><!-- #SmtpPassword -->

        <!-- #SmtpTest -->
        <form-table-row style="background: #fafafa;">
          <template slot="label">
            Send Test E-mail
          </template>
          <template slot="help">
            Test your SMTP configuration by sending a test e-mail to this address. If
            you do not receive an e-mail within a minute, check your settings.
            <br>
            <br>
            <i>You <b>must</b> save any changes for them to take effect.</i>
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="smtpTestEmail">
            <div style="width: 100%; margin-top: 0.5em; text-align: right">
              <button class="ui button tiny" @click="sendTestEmail">
                Send Test E-mail
              </button>
            </div>
          </template>
        </form-table-row><!-- #SmtpPassword -->
        </tbody>
      </table><!-- #E-mail -->
    </div>

    <!-- #VpnTab -->
    <div class="ui tab form" data-tab="vpn" style="padding: 1em;">
      <table class="ui very basic table">
        <tbody>
        <!-- #VpnHostname -->
        <form-table-row>
          <template slot="label">
            Hostname
          </template>
          <template slot="help">
            The hostname or IP address that users will use when connecting to
            the OpenVPN server. This should be a publicly available IP address.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.vpn_hostname">
            <p class="form-error">
              {{ errors.vpn_hostname | error }}
            </p>
          </template>
        </form-table-row><!-- #Name -->

        <!-- #VpnInterface -->
        <form-table-row>
          <template slot="label">
            Interface
          </template>
          <template slot="help">
            The name of the network interface that the OpenVPN server will bind
            to and listen for incoming client connections.
          </template>
          <template slot="input">
            <select id="vpnInterfaceDropdown" class="ui dropdown" v-model="settings.vpn_interface">
              <option :value="iface" v-for="(addr, iface) in settings.interfaces">
                {{ iface }} - {{ addr }}
              </option>
            </select>
            <p class="form-error">
              {{ errors.vpn_interface | error }}
            </p>
          </template>
        </form-table-row><!-- #VpnInterface -->

        <!-- #VpnPort -->
        <form-table-row>
          <template slot="label">
            Port
          </template>
          <template slot="help">
            The port number that the OpenVPN server will bind to and listen for
            incoming client connections.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.vpn_port">
            <p class="form-error">
              {{ errors.vpn_port | error }}
            </p>
          </template>
        </form-table-row><!-- #VpnPort -->

        <!-- #VpnProtocol -->
        <form-table-row>
          <template slot="label">
            Protocol
          </template>
          <template slot="help">
            The network protocol that will be used for communication between
            the OpenVPN server and clients.
          </template>
          <template slot="input">
            <select id="vpnProtocolDropdown" class="ui dropdown" v-model="settings.vpn_protocol">
              <option value="tcp">TCP</option>
              <option value="udp">UDP</option>
            </select>
            <p class="form-error">
              {{ errors.vpn_protocol | error }}
            </p>
          </template>
        </form-table-row><!-- #VpnProtocol -->

        <!-- #VpnSubnet -->
        <form-table-row>
          <template slot="label">
            Client Subnet
          </template>
          <template slot="help">
            The CIDR address for the subnet from which clients will be assigned
            IP addresses. This should be large enough to accomodate the expected
            number of users.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.vpn_subnet">
            <p class="form-error">
              {{ errors.vpn_subnet | error }}
            </p>
          </template>
        </form-table-row><!-- #VpnSubnet -->

        <!-- #VpnRedirectGateway -->
        <form-table-row>
          <template slot="label">
            Redirect Gateway
          </template>
          <template slot="help">
            When set to <i>Yes</i>, all traffic from the client machine will be routed
            through the OpenVPN server, regardless of whether it's destined for a pushed
            network or not.
          </template>
          <template slot="input">
            <select id="vpnRedirectGatewayDropdown" class="ui dropdown" v-model="settings.vpn_redirect_gateway">
              <option value="True">Yes</option>
              <option value="False">No</option>
            </select>
            <p class="form-error">
              {{ errors.vpn_redirect_gateway | error }}
            </p>
          </template>
        </form-table-row><!-- #VpnRedirectGateway -->

        <!-- #VpnNatInterface -->
        <form-table-row>
          <template slot="label">
            NAT Interface
          </template>
          <template slot="help">
            The name of the network interface that the OpenVPN server will use to
            NAT traffic from. All traffic from the VPN will have this as it's source
            IP address when being routed.
          </template>
          <template slot="input">
            <select id="vpnNatInterfaceDropdown" class="ui dropdown" v-model="settings.vpn_nat_interface">
              <option :value="iface" v-for="(addr, iface) in settings.interfaces">
                {{ iface }} - {{ addr }}
              </option>
            </select>
            <p class="form-error">
              {{ errors.vpn_nat_interface | error }}
            </p>
          </template>
        </form-table-row><!-- #VpnNatInterface -->

        <!-- #VpnRoutes -->
        <form-table-row>
          <template slot="label">
            Routes
          </template>
          <template slot="help">
            The list of routes, each on its own line, that will be pushed to
            each OpenVPN client when connecting. Despite being pushed, clients
            will still be restricted based on any firewall rules.
          </template>
          <template slot="input">
            <textarea class="textarea" rows="5" v-model="settings.vpn_routes"></textarea>
            <p class="form-error">
              {{ errors.vpn_routes | error }}
            </p>
          </template>
        </form-table-row><!-- #VpnRoutes -->

        <!-- #VpnNameservers -->
        <form-table-row>
          <template slot="label">
            DNS Servers
          </template>
          <template slot="help">
            The list of DNS servers, each on its own line, that will be pushed to
            each OpenVPN client when connecting. Routes to these servers will be
            automatically pushed.
          </template>
          <template slot="input">
            <textarea class="textarea" rows="5" v-model="settings.vpn_nameservers"></textarea>
            <p class="form-error">
              {{ errors.vpn_nameservers | error }}
            </p>
          </template>
        </form-table-row><!-- #VpnNameservers -->

        <!-- #VpnDomain -->
        <form-table-row>
          <template slot="label">
            Domain
          </template>
          <template slot="help">
            The domain name that will be given to each OpenVPN client.
          </template>
          <template slot="input">
            <input type="text" class="input" v-model="settings.vpn_domain">
            <p class="form-error">
              {{ errors.vpn_domain | error }}
            </p>
          </template>
        </form-table-row><!-- #VpnPort -->
        </tbody>
      </table>
    </div><!-- #VpnTab -->

    <!-- #Actions -->
    <div class="form-actions">
      <a href="/#/admin" class="ui button">
        Cancel
      </a>
      <button class="ui button green" @click="updateSettings">
        Update
      </button>
    </div><!-- #Actions -->
  </div>
</template>

<script>
  import helpers from '@/utils/common'
  import mixins from "@/utils/mixins"
  import FormTableRow from '@/components/common/FormTableRow'

  export default {
    name: "Settings",
    mixins: [mixins.AppComponentMixin, mixins.PaginationMixin, ],
    components: {FormTableRow,},
    data() {
      return {
        activeTab: "app",
        errors: {},
        settings: {},
        smtpTestEmail: "",
      }
    },
    mounted() {
      this.getSettings();

      // Whenever the tab is changed, the settings will be retrieved. This forces
      // the user to update before they leave each tab
      $(".secondary.menu .item").tab({
        onLoad: (tabPath, parameterArray, historyEvent) => {
          this.activeTab = tabPath;
          this.getSettings();
        },
      });
    },
    methods: {
      /**
       * Retrieves and sets the application settings for the active tab.
       * @returns {null}
       */
      getSettings() {
        this.axios.get(`/admin/settings/${this.activeTab}`).then(resp => {
          this.settings = resp.data;

          switch (this.activeTab) {
            case "vpn":
              $("#vpnInterfaceDropdown").dropdown("set selected", this.settings.vpn_interface);
              $("#vpnNatInterfaceDropdown").dropdown("set selected", this.settings.vpn_nat_interface);
              $("#vpnProtocolDropdown").dropdown("set selected", this.settings.vpn_protocol);
              $("#vpnRedirectGatewayDropdown").dropdown("set selected", this.settings.vpn_redirect_gateway);
              break;
            case "auth":
              $("#oauth2Provider").dropdown("set selected", this.settings.oauth2_provider);
              break;
          }
        });
      },

      /**
       * Updates the loaded application settings.
       * @returns {null}
       */
      updateSettings() {
        helpers.replaceNullWithEmptyString(this.settings);

        this.axios.put(`/admin/settings/${this.activeTab}`, this.settings)
          .then(resp => {
            this.toastr.success("The settings have been updated", "Settings Updated");
            this.errors = {};

            // if the app_organization value was updated then make sure it is reflected
            // in the current UI
            if ("app_organization" in this.settings) {
              this.store.appOrganization = this.settings.app_organization;
            }

            if (this.activeTab === "vpn") {
              this.store.vpnRestartPending = true;
            }
          })
          .catch(err => {
            this.errors = err.response.data;
          });
      },

      /**
       * Sends a test e-mail to check SMTP configuration.
       * @returns {null}
       */
      sendTestEmail() {
        this.axios.post("/admin/settings/mail/test", {email: this.smtpTestEmail})
          .then(resp => {
            this.toastr.success("A test e-mail was sent.", "E-mail Sent");
            this.smtpTestEmail = "";
          });
      },

      setupLetsEncrypt() {
        this.axios.post("/admin/settings/letsEncrypt")
          .then(resp => {
            this.toastr.success("SSL has been setup! Refreshing...", "SSL Setup");

            // simulate a sleep for 3s to give the web server time to restart before refreshing
            new Promise(resolve => setTimeout(resolve, 3000))
              .then(() => {
                location.reload(true);
              });
          })
          .catch(err => {
            this.errors.letsEncrypt = err.response.data;
          });
      }
    },
  }
</script>

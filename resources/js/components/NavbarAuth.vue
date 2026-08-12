<template>
  <ul class="navbar-nav ml-auto">
    <li v-if="showDonate" class="nav-item">
      <a class="nav-link" :href="donateUrl">{{ donateLabel }}</a>
    </li>

    <li v-if="loading" class="nav-item" aria-hidden="true">
      <span class="nav-link">&nbsp;</span>
    </li>

    <li v-else-if="!authenticated" class="nav-item">
      <a class="nav-link" :href="loginUrl">{{ loginLabel }}</a>
    </li>

    <template v-else>
      <li class="nav-item">
        <a class="nav-link" :href="postsUrl">{{ postsLabel }}</a>
      </li>
      <li class="nav-item dropdown">
        <a id="navbarDropdown" class="nav-link dropdown-toggle" href="#" role="button"
          data-toggle="dropdown" aria-haspopup="true" aria-expanded="false">
          <img :src="avatarUrl" class="rounded-circle" width="24" height="24"
            style="object-fit: cover" :alt="avatarAlt">
          <span class="caret"></span>
        </a>
        <div class="dropdown-menu dropdown-menu-right" aria-labelledby="navbarDropdown">
          <a class="dropdown-item" :href="profileUrl">{{ profileLabel }}</a>
          <a class="dropdown-item" href="#" @click.prevent="logout">{{ logoutLabel }}</a>
        </div>
      </li>
    </template>
  </ul>
</template>

<script>
export default {
  props: {
    loginUrl: { type: String, required: true },
    postsUrl: { type: String, required: true },
    profileUrl: { type: String, required: true },
    donateUrl: { type: String, required: true },
    defaultAvatarUrl: { type: String, required: true },
    loginLabel: { type: String, required: true },
    postsLabel: { type: String, required: true },
    profileLabel: { type: String, required: true },
    logoutLabel: { type: String, required: true },
    donateLabel: { type: String, required: true },
    avatarAlt: { type: String, required: true },
    showDonate: { type: Boolean, default: false },
  },
  data() {
    return {
      loading: true,
      authenticated: false,
      user: null,
    };
  },
  computed: {
    avatarUrl() {
      return this.user && this.user.avatar_url
        ? this.user.avatar_url
        : this.defaultAvatarUrl;
    },
  },
  mounted() {
    this.loadSession();
  },
  methods: {
    /**
     * Reads the session cookie the Go API set.
     *
     * The readable half of the session: a cross-site request carries the httpOnly refresh
     * cookie automatically but cannot read this one to echo it in the header, which is what
     * makes the header a proof of same-site origin. An absent value means no session was
     * ever established in this browser, so there is nothing to refresh.
     */
    csrfCookie() {
      const match = document.cookie.match(/(?:^|;\s*)2pick_csrf=([^;]*)/);
      return match ? decodeURIComponent(match[1]) : '';
    },
    /**
     * Asks the Go API who is signed in.
     *
     * Two calls rather than one because the session is split in half by design: the refresh
     * cookie buys a short-lived access token, and only that token can read the account. The
     * previous single call to Laravel's `/session-context` read a PHP session that this page
     * no longer has any part in.
     */
    loadSession() {
      const csrfToken = this.csrfCookie();
      if (!csrfToken) {
        this.loading = false;
        return;
      }
      axios.post('/api/v1/auth/refresh', {}, {
        withCredentials: true,
        headers: { 'X-CSRF-Token': csrfToken },
      }).then(response => {
        const grant = response.data.data || {};
        return axios.get('/api/v1/auth/me', {
          headers: { Authorization: 'Bearer ' + grant.access_token },
        });
      }).then(response => {
        const identity = response.data.data || {};
        this.authenticated = true;
        this.user = identity.user || null;
      }).catch(() => {
        // Expired, revoked, or never signed in — all of which mean the same thing here.
        this.authenticated = false;
      }).finally(() => {
        this.loading = false;
      });
    },
    /**
     * Ends the session server-side, then reloads so every part of the page agrees.
     *
     * The CSRF value is re-read rather than remembered: refreshing rotated the session,
     * and the cookie holds the current token.
     */
    logout() {
      const csrfToken = this.csrfCookie();
      axios.post('/api/v1/auth/logout', {}, {
        withCredentials: true,
        headers: { 'X-CSRF-Token': csrfToken },
      }).catch(() => {
        // A failed logout still clears this page's view of the session; the reload below
        // will show signed-out state and the cookie is already unusable if it was revoked.
      }).finally(() => {
        window.location.reload();
      });
    },
  },
};
</script>

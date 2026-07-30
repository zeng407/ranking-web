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
          <form ref="logoutForm" :action="logoutUrl" method="POST" class="d-none">
            <input type="hidden" name="_token" :value="csrfToken">
          </form>
        </div>
      </li>
    </template>
  </ul>
</template>

<script>
export default {
  props: {
    contextEndpoint: { type: String, required: true },
    loginUrl: { type: String, required: true },
    postsUrl: { type: String, required: true },
    profileUrl: { type: String, required: true },
    logoutUrl: { type: String, required: true },
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
      csrfToken: '',
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
    axios.get(this.contextEndpoint, {
      headers: { 'Cache-Control': 'no-cache' },
    }).then(response => {
      this.authenticated = response.data.authenticated === true;
      this.csrfToken = response.data.csrf_token || '';
      this.user = response.data.user;
    }).catch(() => {
      this.authenticated = false;
    }).finally(() => {
      this.loading = false;
    });
  },
  methods: {
    logout() {
      if (this.csrfToken) {
        this.$refs.logoutForm.submit();
      }
    },
  },
};
</script>

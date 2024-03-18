<script>

export default {
  props: {
    propsNickname: {
      type: String,
      required: true
    },
    propsEmail: {
      type: String,
      required: true
    },
    propsAvatarUrl: {
      type: String,
      required: true
    },
    nicknameMaxLength: {
      type: String,
      required: true
    }
  },
  data() {
    return {
      nickname: this.propsNickname,
      email: this.propsEmail,
      maskEmail: true,
      avatarUrl: this.propsAvatarUrl,
      isAvatarChanged: false
    };
  },
  computed: {
    nicknameLength() {
      return this.nickname.length;
    },
    isNicknameChanged() {
      return this.nickname !== this.propsNickname;
    },
    isEmailChanged() {
      return this.email !== this.propsEmail;
    },
    maskedEmail() {
      if (!this.email) return '';
      const [user, domain] = this.email.split('@');
      const maskedUser = '*'.repeat(10);
      const partialUser = user.slice(0, 2);
      return `${partialUser}${maskedUser}@${domain}`;
    },
  },
  methods: {
    toggleMaskEmail() {
      this.maskEmail = !this.maskEmail; // toggle the maskEmail property
    },
    uploadAvatar() {
      const fileInput = document.getElementById('avatar-upload');
      fileInput.click();
    },
    handleAvatarChange(event) {
      this.isAvatarChanged = true;
      const file = event.target.files[0];
      const reader = new FileReader();
      reader.onload = (e) => {
        this.avatarUrl = e.target.result;
      };
      reader.readAsDataURL(file);
    }
  },
};

</script>
<script>

export default {
  components: {
  },
  mounted() {
    $('#modal').modal('show');
  },
  data() {
    return {
      inputPassword: '',
      isInvalidPassword: false,
    }
  },
  props: {
    postSerial: {
      type: String,
      required: true
    },
    accessEndpoint: {
      type: String,
      required: true
    },
    redirectUrl: {
      type: String,
      required: true
    }
  },
  computed: {
    
  },
  methods: {
    submitPassword() {
      // set header Authorization
      if(this.inputPassword){
        axios.defaults.headers.common['Authorization'] = this.inputPassword;
      }else{
        this.isInvalidPassword = true;
        return;
      }

      axios.get(this.accessEndpoint)
      .then(response => {
        if (response.status === 200) {
          window.location.href = this.redirectUrl;
        } else {
          this.isInvalidPassword = true;
          // Swal.fire({
          //   icon: 'error',
          //   position: "top-end",
          //   showConfirmButton: false,
          //   toast: true,
          //   text: this.$t('game.invalid_password'),
          // });
        }
      })
      .catch(error => {
        if(error.response.status === 403){
          this.isInvalidPassword = true;
        }else{
          console.log(error);
        }

      });
    }
  }
}
</script>

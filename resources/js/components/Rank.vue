<script>
import Swal from 'sweetalert2';
import CountWords from './partials/CountWords.vue';
export default {
  components: {
    CountWords
  },
  mounted() {
    this.loadCommnets();

    if (window.adsbygoogle) {
      setTimeout(() => {
        try{
          window.adsbygoogle.push({});
          window.adsbygoogle.push({});
        }catch(e){}
      }, 500);

      setTimeout(() => {
        if (window.adsbygoogle) {
          if($('#google-ad-1')) {
          $('#google-ad-1').addClass('d-flex justify-content-center');
          }
        }
      }, 1000);
    }
  },
  data() {
    return {
      commentInput: '',
      comments: [],
      meta: {
        current_page: 1,
        last_page: 1
      },
      profile: {
        nickname: '',
        avatar_url: '',
        champions: []
      }
    }
  },
  props: {
    commentMaxLength: {
      type: String,
      required: true
    },
    indexCommentEndpoint: {
      type: String,
      required: true
    },
    createCommentEndpoint: {
      type: String,
      required: true
    },
    reportCommentEndpoint: {
      type: String,
      required: true
    }
  },
  computed: {
    commentWords() {
      return this.commentInput.length;
    },
    validComment() {
      return this.commentInput.trim().length > 0 && this.commentInput.length <= this.commentMaxLength
    }
  },
  methods: {
    loadCommnets: function (page = 1) {
      const urlParams = {
        page: page
      }
      axios.get(this.indexCommentEndpoint, {
        params: urlParams
      })
        .then(response => {
          this.comments = response.data.data;
          this.meta = response.data.meta;
          this.profile = response.data.profile;
        })
        .catch(error => {
          // console.log(error);
          Swal.fire({
            title: 'Error!',
            text: this.$t('Something went wrong. Please try again later.'),
            icon: 'error',
          });
        });
    },
    clickTab: function (tab) {
      const urlParams = new URLSearchParams(window.location.search);
      urlParams.set('tab', tab);
      const newUrl = window.location.pathname + '?' + urlParams.toString();
      window.history.replaceState(null, null, newUrl);
    },
    submitComment() {
      if (this.commentInput.length > 0 && this.commentInput.length <= this.commentMaxLength) {
        axios.post(this.createCommentEndpoint, {
          content: this.commentInput
        })
          .then(response => {
            this.commentInput = '';
            this.loadCommnets();
            // Scroll to the comment position
            const navbarHeight = 60;
            $("html, body").animate({ scrollTop: $('#comments-total').offset().top - navbarHeight }, 1000);
          })
          .catch(error => {
            // console.log(error);
            Swal.fire({
              title: 'Error!',
              text: error.response.data.message,
              icon: 'error',
              button: 'OK'
            });
          });
      }
    },
    changePage: function (page) {
      this.loadCommnets(page);
    },
    reportComment: function (comment) {
      const commentId = comment.id;

      Swal.fire({
        title: this.$t('Are you sure?'),
        text: 'Are you sure you want to report this comment?',
        icon: 'warning',
        showCancelButton: true,
        confirmButtonText: 'Yes',
        cancelButtonText: 'No',
        html: `Comment: <br><b> ${comment.content} </b><br> <strong>By: <i>${comment.nickname}</i></strong>`,
        input: 'select',
        inputOptions: {
          'Spam': this.$t('Spam'),
          'Inappropriate': this.$t('Inappropriate'),
          'Hate Speech': this.$t('Hate Speech'),
          'Harassment': this.$t('Harassment'),
          'Other': this.$t('Other')
        },
        inputPlaceholder: this.$t("Please select a reason for reporting"),
      }).then((result) => {
        if (result.isConfirmed) {
          if (result.value === 'Other') {
            Swal.fire({
              title: this.$t('Please specify the reason'),
              input: 'text',
              inputPlaceholder: this.$t('Please specify the reason'),
              showCancelButton: true,
              confirmButtonText: this.$t('Submit'),
              cancelButtonText: this.$t('Cancel'),
              inputValidator: (value) => {
                if (!value) {
                  return this.$t('report_comment_other_reason_required');
                }
              }
            }).then((result) => {
              if (result.isConfirmed) {
                const reportReason = result.value;
                const payload = {
                  reason: reportReason
                }
                this.performReportingComment(commentId, payload);
              }
            });
            return;
          } else {
            const reportReason = result.value;
            const payload = {
              reason: this.$t(reportReason)
            }
            this.performReportingComment(commentId, payload);
          }
        }
      });
    },
    performReportingComment: function (commentId, payload) {
      axios.post(this.reportCommentEndpoint.replace('_comment_id', commentId), payload)
        .then(response => {
          Swal.fire({
            title: this.$t('Reported!'),
            icon: 'success'
          });
        })
        .catch(error => {
          // console.log(error);
          Swal.fire({
            title: 'Error!',
            text: this.$t('Something went wrong. Please try again later.'),
            icon: 'error',
          });
        });
    },
    share: function() {
      const url = window.location.origin + window.location.pathname + '?utm_medium=share_rank';
      if (navigator.share) {
        navigator.share({
          url: url
        }).catch(console.error);
      }else{  
        this.$refs['popover'].$emit('open');
        navigator.clipboard.writeText(url);
        setTimeout(() => {
          this.$root.$emit('bv::hide::popover');
        }, 2000);
      }
    },
    shareResult: function() {

      //get parameter g
      const urlParams = new URLSearchParams(window.location.search);
      const g = urlParams.get('g');
      const url = window.location.origin + window.location.pathname + '?s=' + g + '&utm_medium=share_result';

      if (navigator.share) {
        navigator.share({
          url: url,
          title: this.$t('My Voting Game Result'),
          text: this.$t('Check out my result on this voting game!'),
        }).catch(console.error);
      }else{  
        this.$refs['share-popover'].$emit('open');
        navigator.clipboard.writeText(url);
        setTimeout(() => {
          this.$root.$emit('bv::hide::popover');
        }, 2000);
      }
    }
  }
}
</script>

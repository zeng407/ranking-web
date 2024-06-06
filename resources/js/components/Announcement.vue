<script>
import Swal from 'sweetalert2';
export default {
  mounted() {
    this.showAnnouncement();
  },
  props: {
    announcement: Object,
  },
  data: function () {
    return {

    }
  },
  watch: {

  },
  methods: {
    showAnnouncement() {
      if (this.isReadBefore() || !this.announcement.content) {
        return;
      }
      Swal.fire({
        title: this.$t('Announcement'),
        text: this.announcement.content,
        imageUrl: this.announcement.image_url,
        imageWidth: 'auto',
        imageHeight: 200,
        confirmButtonText: this.$t('Never show again'),
        confirmButtonColor: '#000',
        allowOutsideClick: true,
        allowEscapeKey: true,
        allowEnterKey: true,
      }).then((result) => {
        if (result.isConfirmed) {
          this.closeAnnouncement();
        }
      });
    },
    isReadBefore() {
      let announcement = this.$cookies.get('announcement');
      return announcement == this.announcement.id;
    },
    closeAnnouncement() {
      let minutes = this.announcement.keep_minutes;
      this.$cookies.set('announcement', this.announcement.id, `${minutes} minutes`);
    },
  }
}
</script>

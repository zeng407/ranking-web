<template>
  <div>
    <div class="modal fade" id="updateHomeCarouselModal" tabindex="-1" role="dialog" data-backdrop="static"
      data-keyboard="false" aria-labelledby="updateHomeCarouselModalLabel" aria-hidden="true">
      <div class="modal-dialog" role="document">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title" id="updateHomeCarouselModalLabel">Update Home Carousel Item</h5>
            <button type="button" class="close" data-dismiss="modal" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
          <form @submit.prevent="updateHomeCarousel">
            <div class="modal-body">
              <div class="row">
                <div class="form-group col-12">
                  <label for="type" class="col-sm-2 col-form-label">Type</label>
                  <div class="col-sm-10">
                    <input type="text" class="form-control"v-model="type" disabled>
                  </div>
                </div>
              </div>
              <div class="row">
                <div v-if="type === 'image'" class="form-group col-12">
                  <label for="image" class="col-sm-2 col-form-label">Image</label>
                  <div class="col-sm-10">
                    <input type="text" class="form-control"v-model="image" disabled>
                  </div>
                </div>
                <div v-if="type === 'video'" class="form-group col-12">
                  <label for="video" class="col-sm-2 col-form-label">Video</label>
                  <div class="col-sm-10">
                    <input type="text" class="form-control" v-model="video" disabled>
                  </div>
                </div>
                <div class="form-group col-12">
                  <label for="title" class="col-sm-2 col-form-label">Title</label>
                  <div class="col-sm-10">
                    <input type="text" class="form-control" id="title" v-model="title" placeholder="Title"
                      autocomplete="off">
                  </div>
                </div>
                <div class="form-group col-12">
                  <label for="description" class="col-sm-2 col-form-label">Description</label>
                  <div class="col-sm-10">
                    <textarea class="form-control" id="description" v-model="description"
                      placeholder="Description"></textarea>
                  </div>
                </div>
                <div v-if="type === 'video'" class="form-group col-12">
                  <label for="video_start_second" class="col-sm-2 col-form-label">Start Seconds</label>
                  <div class="col-sm-10">
                    <input type="text" class="form-control" id="video_start_second" v-model="video_start_second"
                      placeholder="Start Seconds" autocomplete="off">
                  </div>
                </div>
                <div v-if="type === 'video'" class="form-group col-12">
                  <label for="video_end_second" class="col-sm-2 col-form-label">End Seconds</label>
                  <div class="col-sm-10">
                    <input type="text" class="form-control" id="video_end_second" v-model="video_end_second"
                      placeholder="End Seconds" autocomplete="off">
                  </div>
                </div>
                <div class="modal-footer">
                  <button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>
                  <button type="submit" class="btn btn-primary">Update</button>
                </div>
              </div>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<script>

import Swal from 'sweetalert2';

export default {
  mounted() {
    $('#updateHomeCarouselModal').modal('show');
    $('#updateHomeCarouselModal').on('hidden.bs.modal', this.handleClose);
    this.loadItem();
  },
  beforeDestroy() {
    $('#updateHomeCarouselModal').off('hidden.bs.modal', this.handleClose);
  },
  props: {
    updateEndpoint: {
      type: String,
      required: true
    },
    close: {
      type: Function,
      required: true
    },
    item: {
      type: Object,
      required: true
    }
  },
  data() {
    return {
      image: '',
      video: '',
      title: '',
      description: '',
      type: 'video',
      video_start_second: '',
      video_end_second: '',
    }
  },
  methods: {
    updateHomeCarousel() {
      // transfer timeformat to seconds
      if (this.type === 'video') {
        this.video_start_second = this.timeToSeconds(this.video_start_second);
        this.video_end_second = this.timeToSeconds(this.video_end_second);
      }

      const data = {
        type: this.type,
        image_url: this.image,
        video_url: this.video,
        title: this.title,
        description: this.description,
        video_start_second: this.video_start_second,
        video_end_second: this.video_end_second,
      };
      const url = this.updateEndpoint.replace('_id', this.item.id);

      axios.put(url, data)
        .then(response => {
          console.log(response.data);
          Swal.fire({
            icon: 'success',
            title: 'Success',
            text: 'Home carousel item updated successfully',
          });
          this.handleClose();
        })
        .catch(error => {
          console.log(error.response.data);
          Swal.fire({
            icon: 'error',
            title: 'Error',
            text: 'Unable to update home carousel item',
          });
        });
    },
    handleClose() {
      // hide this modal
      $('#updateHomeCarouselModal').modal('hide');
      this.close();
    },
    loadItem() {
      this.image = this.item.image_url;
      this.video = this.item.video_url;
      this.title = this.item.title;
      this.description = this.item.description;
      this.type = this.item.type;
      this.video_start_second = this.formatTime(this.item.video_start_second);
      this.video_end_second = this.formatTime(this.item.video_end_second);
    },
    resetForm() {
      this.image = '';
      this.video = '';
      this.title = '';
      this.description = '';
      this.type = 'video';
      this.video_start_second = '';
      this.video_end_second = '';
    },
    formatTime: function (time) {
      if(time === null || time == undefined) return '';
      // format second to 0h0m0s
      let hour = Math.floor(time / 3600);
      let minute = Math.floor((time % 3600) / 60);
      let second = time % 60;
      
      return `${hour}:${minute}:${second}`;
    },
    timeToSeconds: function (time) {
      // transfer time format to seconds
      return time.split(':').reduce((acc, time) => (60 * acc) + +time);
    }
  },
}
</script>
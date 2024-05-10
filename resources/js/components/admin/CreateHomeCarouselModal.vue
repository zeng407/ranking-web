<template>
  <div>
    <div class="modal fade" id="createHomeCarouselModal" tabindex="-1" role="dialog" data-backdrop="static"
      data-keyboard="false" aria-labelledby="createHomeCarouselModalLabel" aria-hidden="true">
      <div class="modal-dialog" role="document">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title" id="createHomeCarouselModalLabel">Create Home Carousel Item</h5>
            <button type="button" class="close" data-dismiss="modal" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
          <form @submit.prevent="createHomeCarousel">
            <div class="modal-body">
              <div class="row">
                <div class="form-group col-12">
                  <label for="type" class="col-sm-2 col-form-label">Type</label>
                  <div class="col-sm-10">
                    <select class="form-control" id="type" v-model="type" required>
                      <option value="image">Image</option>
                      <option value="video">Video</option>
                    </select>
                  </div>
                </div>
              </div>
              <div class="row">
                <div v-if="type === 'image'" class="form-group col-12">
                  <label for="image" class="col-sm-2 col-form-label">Image</label>
                  <div class="col-sm-10">
                    <input type="text" class="form-control" id="image" v-model="image" placeholder="Image URL"
                      autocomplete="off" required>
                  </div>
                </div>
                <div v-if="type === 'video'" class="form-group col-12">
                  <label for="video" class="col-sm-2 col-form-label">Video</label>
                  <div class="col-sm-10">
                    <input type="text" class="form-control" id="video" v-model="video" placeholder="Video URL"
                      autocomplete="off" required>
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
                  <button type="submit" class="btn btn-primary">Create</button>
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
    $('#createHomeCarouselModal').modal('show');
    $('#createHomeCarouselModal').on('hidden.bs.modal', this.handleClose);
  },
  beforeDestroy() {
    $('#createHomeCarouselModal').off('hidden.bs.modal', this.handleClose);
  },
  props: {
    createEndpoint: {
      type: String,
      required: true
    },
    close: {
      type: Function,
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
    createHomeCarousel() {

      //transfer time format to seconds
      if (this.type === 'video') {
        this.video_start_second = this.video_start_second.split(':').reduce((acc, time) => (60 * acc) + +time);
        this.video_end_second = this.video_end_second.split(':').reduce((acc, time) => (60 * acc) + +time);
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

      axios.post(this.createEndpoint, data)
        .then(response => {
          console.log(response.data);
          Swal.fire({
            icon: 'success',
            title: 'Success',
            text: 'Home carousel item created successfully',
          });
          this.handleClose();
        })
        .catch(error => {
          console.log(error.response.data);
          Swal.fire({
            icon: 'error',
            title: 'Error',
            text: 'Unable to create home carousel item',
          });
        });
    },
    handleClose() {
      // hide this modal
      $('#createHomeCarouselModal').modal('hide');
      this.close();
    }
  },
}
</script>
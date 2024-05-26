<template>
  <div class="d-inline">
    <a class="btn btn-outline-primary fa-pull-right" @click="showModal">
      <i class="fa fa-edit"></i>
    </a>
    <!-- modal -->
    <div class="modal fade" :id="'editElementModal' + elementId" tabindex="-1" role="dialog"
      :aria-labelledby="'editElementModalLabel'+elementId" aria-hidden="true">
      <div class="modal-dialog" role="document">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title" :id="'editElementModalLabel'+elementId">{{ $t('Edit') }}</h5>
            <button type="button" class="close" data-dismiss="modal" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
          <div class="modal-body text-left">
            <div v-if="preview_image_url" class="row m-1">
              <div class="col-12">
                <button :id="'delete-button' + elementId" class="position-absolute btn btn-danger" style="right: 0%;"
                  @click="deleteFile"><i class="fas fa-times"></i></button>
                <b-popover ref="delete-hint" :target="'delete-button' + elementId">{{ $t('Please delete first, then upload the URL')}}</b-popover>
                <img v-if="is_image" class="w-100" :src="preview_image_url">
                <video v-else class="w-50" controls :src="preview_image_url"></video>
              </div>
            </div>
            <div class="row">
              <div class="col-12">
                <label :for="'uplaodFile' + elementId"><i class="fa-solid fa-upload"></i>&nbsp;{{$t('edit.element.upload-image') }}</label>
                <div class="form-group">
                  <div class="custom-file form-group">
                    <input :id="'uplaodFile' + elementId" @change="uplaodFile" type="file" accept="image/*,video/*,audio/*" class="custom-file-input">
                    <label class="custom-file-label" :for="'uplaodFile' + elementId">{{ $t('Choose File...') }}</label>
                  </div>
                </div>
              </div>
              <div class="col-12">
                <label :for="'elementSourceUrl' + elementId">
                  <i class="fa-solid fa-photo-film"></i>&nbsp;{{ $t('edit_post.upload_batch') }}
                </label>
                <div class="form-group white-space-normal">
                  <input type="text" class="form-control" :id="'elementSourceUrl' + elementId" v-model="url"
                    :readonly="preview_image_url" @click="hintDeleteFirst"> </input>
                  <small class="break-all">{{ $t('Current URL') }}:&nbsp;{{sourceUrl}}</small>
                  <button :id="'popover-copy-taget' + elementId" type="button" class="btn btn-sm btn-outline-dark" @click="copyUrl"><i class="fa-xs fa-regular fa-copy"></i></button>
                  <b-popover :ref="'popover-copy' + elementId" :target="'popover-copy-taget' + elementId" :disabled="true">
                    {{$t('Copied')}}
                  </b-popover>

                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-dismiss="modal">{{ $t('Close') }}</button>
            <button type="button" class="btn btn-primary" @click="updateElement" :disabled="saving">{{ $t('Save changes') }} <i v-show="saving" class="fas fa-spinner fa-spin"></i></button>
          </div>
        </div>
      </div>
    </div>
    <!-- end modal -->
  </div>

</template>

<script>
import Swal from 'sweetalert2';

export default {
  mounted() {

  },
  props: {
    postSerial: {
      type: String,
      required: true
    },
    elementId: {
      type: String,
      required: true
    },
    sourceUrl: {
      type: String,
      required: true
    },
    updateElementRoute: {
      type: String,
      required: true
    },
    uploadElementRoute: {
      type: String,
      required: true
    }
  },
  data() {
    return {
      path_id: null,
      preview_image_url: null,
      url: null,
      saving: false,
      is_image: true
    }
  },
  computed: {
    dataChanged() {
      return this.preview_image_url !== null
        || this.url !== null;
    }
  },
  methods: {
    showModal() {
      $(`#editElementModal${this.elementId}`).modal('show');
    },
    updateElement() {
      this.saving = true;
      const route = this.updateElementRoute.replace('_id', this.elementId);
      let data = {
        post_serial: this.postSerial,
        path_id: this.path_id,
        url: this.url
      };

      //remove null values
      Object.keys(data).forEach(key => (data[key] == null || data[key] == "") && delete data[key]);
      axios.put(route, data)
        .then(response => {
          $(`#editElementModal${this.elementId}`).modal('hide');
          this.$emit('elementUpdated', response.data);
          this.url = null;
          this.path_id = null;
          this.preview_image_url = null;
        })
        .catch(error => {
          console.error(error);
          //show error message
          Swal.fire({
            icon: 'error',
            title: this.$t('Oops...'),
            text: error.response.data.message
          });
        })
        .finally(() => {
          this.saving = false;
        })
    },
    uplaodFile(event) {
      const file = event.target.files[0];
      const formData = new FormData();
      formData.append('file', file);
      formData.append('post_serial', this.postSerial);

      const route = this.uploadElementRoute.replace('_id', this.elementId);
      axios.post(route, formData)
        .then(response => {
          this.path_id = response.data.path_id;
          this.preview_image_url = response.data.url;
          this.is_image = response.data.is_image;
        })
        .catch(error => {
          console.error(error);
        });

      // reset file input so that the same file can be uploaded again
      event.target.value = '';
    },
    hintDeleteFirst: function () {
      if (this.preview_image_url && this.$refs['delete-hint']) {
        this.$refs['delete-hint'].$emit('open');

        //scroll to target 
        const hight = 80;
        $("#editElementModal" + this.elementId).animate({ scrollTop: hight }, 500);
        setTimeout(() => {
          this.$root.$emit('bv::hide::popover');
        }, 3500);

      }
    },
    deleteFile: function () {
      this.preview_image_url = null;
      this.path_id = null;
    },
    copyUrl() {
      navigator.clipboard.writeText(this.sourceUrl)
        .then(() => {
          this.$refs['popover-copy' + this.elementId].$emit('open');
          setTimeout(() => {
            this.$root.$emit('bv::hide::popover');
          }, 2000);
        })
    }
  }
}
</script>

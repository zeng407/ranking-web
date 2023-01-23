<template>

  <!--  Main -->
  <div class="container-fluid">

    <!-- Alert -->
    <div class="position-fixed"
         style="top: 20px; right: 15px; z-index: 10">
      <span class="cursor-pointer" @click="dismissAlert">
        <b-alert
          :show="dismissCountDown"
          @dismissed="dismissCountDown=0"
          dismissible
          fade
          :variant="alertLevel"
        >
          {{ alertText }}
        </b-alert>
      </span>
    </div>

    <!-- tabs -->
    <div class="row">

      <div class="col-2">
        <div class="nav flex-column nav-pills" id="v-pills-tab" role="tablist" aria-orientation="vertical">

          <a class="nav-link active" id="v-pills-post-info-tab" data-toggle="pill" href="#v-pills-post-info" role="tab"
             aria-controls="v-pills-post-info" aria-selected="true">
            <span class="d-inline-block text-center" style="width: 30px">
              <i class="fas fa-info-circle"></i>
            </span>
            {{ $t('edit_post.tab.info') }}
          </a>
          <a class="nav-link" id="v-pills-elements-tab" data-toggle="pill" href="#v-pills-elements" role="tab"
             aria-controls="v-pills-elements" aria-selected="false">
            <span class="d-inline-block text-center" style="width: 30px">
              <i class="fas fa-photo-video"></i>
            </span>
            {{ $t('edit_post.tab.element') }}
          </a>
          <a class="nav-link" id="v-pills-rank-tab" data-toggle="pill" href="#v-pills-rank" role="tab"
             aria-controls="v-pills-rank" aria-selected="false">
            <span class="d-inline-block text-center" style="width: 30px">
              <i class="fas fa-trophy"></i>
            </span>
            {{ $t('edit_post.tab.rank') }}
          </a>
        </div>
      </div>

      <div class="col-10">
        <div class="tab-content" id="v-pills-tabContent">

          <!-- tab info -->
          <div class="tab-pane fade show active" id="v-pills-post-info" role="tabpanel"
               aria-labelledby="v-pills-post-info-tab">
            <!-- Loading -->
            <div class="col-9 fa-3x text-center" v-if="loading['LOADING_POST']">
              <i class="fas fa-spinner fa-spin"></i>
            </div>

            <ValidationObserver v-slot="{ invalid }" v-if="post">
              <div class="row">
                <div class="col-6">
                  <h2 class="mt-3 mb-3">{{ $t('edit_post.info.head') }}</h2>
                </div>
                <div class="col-6">
                  <h2 class="mt-3 mb-3">
                  <span class="d-flex justify-content-end">
                    <a class="btn btn-danger mr-3" :href="playGameRoute" target="_blank">
                      <i class="fas fa-play"></i> {{ $t('edit_post.info.play') }}
                    </a>

                    <button class="btn btn-primary" v-if="!isEditing" @click="clickEdit">
                      <i class="fas fa-edit"></i> {{ $t('edit_post.info.edit') }}
                    </button>
                    <button class="btn btn-primary" v-else
                            @click="savePost" :disabled="invalid || loading['SAVING_POST']">
                      <i class="fas fa-save" v-if="!loading['SAVING_POST']"></i>
                      <i class="fas fa-spinner fa-spin" v-if="loading['SAVING_POST']"></i>
                      {{ $t('edit_post.info.save') }}
                    </button>
                  </span>
                  </h2>
                </div>
              </div>
              <form @submit.prevent>
                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg" for="title">
                        {{ $t('Title') }}
                      </label>
                      <ValidationProvider rules="required" v-slot="{ errors }">
                        <input type="text" class="form-control" id="title" v-model="post.title" required
                               :disabled="!isEditing || loading['SAVING_POST']" maxlength="40"
                        >
                        <span class="text-danger">{{ errors[0] }}</span>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>
                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg" for="description">
                        {{ $t('Description') }}
                      </label>
                      <ValidationProvider rules="required" v-slot="{ errors }">
                        <textarea class="form-control" id="description" v-model="post.description" rows="3"
                                  style="resize: none"
                                  maxlength="100"
                                  aria-describedby="description-help"
                                  required
                                  :disabled="!isEditing || loading['SAVING_POST']">

                        </textarea>
                        <small id="description-help" class="form-text text-muted">
                          {{ $t('create_game.description.hint') }}
                        </small>
                        <span class="text-danger">{{ errors[0] }}</span>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>
                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg">
                        {{ $t('Publish') }}
                      </label>
                      <ValidationProvider rules="required" v-slot="{ errors }" v-if="isEditing">
                        <label class="form-control btn btn-outline-dark" for="post-privacy-public">
                          <input type="radio" id="post-privacy-public" v-model="post.policy" value="public"
                                 :disabled="loading['SAVING_POST']"
                          >

                          {{ $t('Public') }}
                        </label>

                        <label class="form-control btn btn-outline-dark" for="post-privacy-private">
                          <input type="radio" id="post-privacy-private" v-model="post.policy" value="private"
                                 :disabled="loading['SAVING_POST']"
                          >
                          {{ $t('Private') }}
                        </label>
                        <span class="text-danger">{{ errors[0] }}</span>
                      </ValidationProvider>
                      <div v-else>
                        <input class="form-control" disabled="disabled" :value="post._.policy">
                        <small class="form-text text-muted"
                               v-if="post.policy==='public'">{{ $t('edit_post.at_least_element_number_hint') }}</small>
                      </div>
                    </div>
                  </div>
                </div>
              </form>
            </ValidationObserver>
            <div class="row" v-if="post">
              <div class="col-3">
                <div class="form-group">
                  <label class="col-form-label-lg">{{ $t('edit_post.info.create_time') }}</label>
                  <input class="form-control" disabled :value="post.created_at | date">
                </div>
              </div>
            </div>
          </div>

          <!-- tab elements -->
          <div class="tab-pane fade" id="v-pills-elements" role="tabpanel" aria-labelledby="v-pills-elements-tab">
            <!-- upload image from device -->
            <h2 class="mt-3 mb-3">{{ $t('edit_post.upload_image') }}</h2>
            <div class="row">
              <div class="col-12">
                <label for="image-upload">從電腦上傳</label>
                <div class="custom-file form-group">
                  <input type="file" class="custom-file-input" id="image-upload" multiple
                         @change="uploadImages">
                  <label class="custom-file-label" for="image-upload">Choose File...</label>
                </div>
              </div>
            </div>
            <!-- upload progress bar -->
            <div class="progress mb-1" v-for="(progress, name) in uploadingFiles">
              <div class="progress-bar progress-bar-striped progress-bar-animated" role="progressbar" aria-valuemin="0"
                   aria-valuemax="100"
                   :aria-valuenow="progress" :style="{'width': progress+'%'}">
                {{ name }}
              </div>
            </div>
            <!-- upload image from url -->
            <div class="row mt-3">
              <div class="col-12">
                <label for="image-url-upload">從url上傳</label>
                <div class="input-group">
                  <input class="form-control" type="text" id="image-url-upload" name="image-url-upload"
                         v-model="imageUrl"
                         placeholder="https://imgur.com/gallery/YdaUbzZ">
                  <div class="input-group-append">
                    <button class="btn btn-outline-secondary" type="button" @click="uploadImageUrl">
                      <span v-show="!loading['UPLOADING_IMAGE']">{{ $t('edit_post.add_image_button') }}</span>
                      <i v-show="loading['UPLOADING_IMAGE']" class="fas fa-spinner fa-spin"></i>
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <!-- upload video -->
            <h2 class="mt-5 mb-3">{{ $t('edit_post.upload_video') }}</h2>
            <div class="row">
              <div class="col-12">
                <label for="youtubeURL">Youtube</label>
                <div class="input-group">
                  <input class="form-control" type="text" id="youtubeURL" name="youtubeURL"
                         v-model="videoUrl"
                         aria-label="https://www.youtube.com/watch?v=dQw4w9WgXcQ" aria-describedby="youtubeUpload"
                         placeholder="https://www.youtube.com/watch?v=dQw4w9WgXcQ">
                  <div class="input-group-append">
                    <button class="btn btn-outline-secondary" type="button" id="youtubeUpload" @click="uploadVideo"
                            :disabled="loading['UPLOADING_VIDEO']">
                      <span v-show="!loading['UPLOADING_VIDEO']">{{ $t('edit_post.add_video_button') }}</span>
                      <i v-show="loading['UPLOADING_VIDEO']" class="fas fa-spinner fa-spin"></i>

                      <!--                      <i class="fas fa-spinner"></i>-->
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <!-- edit -->
            <h2 class="mt-5 mb-3">{{ $t('edit_post.edit_media') }}</h2>

            <nav class="navbar navbar-light bg-light pr-0 justify-content-end">
              <div class="form-inline">
                <input class="form-control mr-sm-2" v-model="filters.title_like" type="search" placeholder="Search"
                       aria-label="Search">
                <i class="fas fa-filter" v-if="filters.title_like"></i>
              </div>
            </nav>


            <div class="row">
              <template v-for="(element, index) in filteredElements">
                <div class="col-lg-4 col-md-6" v-show="element.type==='video' && isElementInPage(index)">
                  <div class="card mb-3">
                    <!-- video -->
                    <youtube v-if="element.type==='video' && element.loadedVideo" width="100%" height="270"
                             :ref="element.id"
                             @ready="doPlay(element)"
                             :player-vars="{
                              controls:1,
                              autoplay:0,
                              start: element.video_start_second,
                              end:element.video_end_second,
                              rel: 0,
                              host: 'https://www.youtube.com'
                              }"
                    ></youtube>
                    <img :src="element.thumb_url" class="card-img-top" :alt="element.title"
                         v-if="element.type==='video' && !element.loadedVideo">

                    <div class="card-body">
                      <input class="form-control-plaintext bg-light cursor-pointer" type="text" :value="element.title"
                             maxlength="100"
                             @change="updateElementTitle(element.id, $event)">
                      <div class="row mb-3">
                        <div class="col-10">
                          <div class="input-group">
                            <div class="input-group-prepend d-lg-none d-xl-block">
                              <span class="input-group-text">{{ $t('edit_post.video_range') }}</span>
                            </div>
                            <input type="text" class="form-control" name="video_start_second"
                                   placeholder="0:00" aria-label="start"
                                   @change="updateVideoScope(index, element, $event)"
                                   :value="toTimeFormat(element.video_start_second)">
                            <div class="input-group-prepend"><span class="input-group-text">~</span></div>
                            <input type="text" class="form-control" name="video_end_second"
                                   :placeholder="toTimeFormat(element.video_duration_second)" aria-label="end"
                                   :value="toTimeFormat(element.video_end_second)"
                                   @change="updateVideoScope(index, element, $event)">
                          </div>
                        </div>
                        <div class="col-2">
                          <a class="btn btn-danger float-right" @click="clickPlayButton(index, element)">
                            <i class="fas fa-play-circle"></i>
                          </a>
                        </div>
                      </div>

                      <span class="card-text"><small
                        class="text-muted">{{ element.created_at | datetime }}</small></span>
                      <a class="btn btn-danger float-right" @click="deleteElement(element)">
                        <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                        <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                      </a>
                    </div>
                  </div>
                </div>

                <!-- image -->
                <div class="col-lg-4" v-show="element.type==='image' && isElementInPage(index)">
                  <div class="card mb-3">
                    <img :src="element.thumb_url" class="card-img-top" :alt="element.title"
                         v-if="element.type==='image'">

                    <div class="card-body">
                      <input class="form-control-plaintext bg-light cursor-pointer mb-2" type="text"
                             :value="element.title"
                             maxlength="100"
                             @change="updateElementTitle(element.id, $event)">
                      <span class="card-text"><small
                        class="text-muted">{{ element.created_at | datetime }}</small></span>
                      <a class="btn btn-danger float-right" @click="deleteElement(element)">
                        <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                        <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                      </a>
                    </div>
                  </div>
                </div>
              </template>
            </div>

            <b-pagination
              v-model="currentPage"
              :total-rows="totalRow"
              :per-page="perPage"
              first-number
              last-number
              align="center"
            ></b-pagination>
          </div>

          <!-- tab rank -->
          <div class="tab-pane fade" id="v-pills-rank" role="tabpanel" aria-labelledby="v-pills-rank-tab">
            <div class="row" v-if="!loading['LOADING_POST']">
              <div class="col-3">
                <div class="form-group">
                  <label>{{ $t('edit_post.rank.game_plays') }}</label>
                  <input class="form-control" disabled :value="post.play_count">
                </div>
              </div>
            </div>
            <table class="table table-hover" style="table-layout: fixed">
              <thead>
              <tr>
                <th scope="col">#</th>
                <th scope="col" style="width: 70%"></th>
                <th scope="col">{{ $t('edit_post.rank.win_at_final') }}</th>
                <th scope="col">{{ $t('edit_post.rank.win_rate') }}</th>
              </tr>
              </thead>
              <tbody v-if="loading['LOADING_RANK']">
              <tr>
                <td colspan="4">
                  <div class="fa-3x text-center">
                    <i class="fas fa-spinner fa-spin"></i>
                  </div>
                </td>
              </tr>
              </tbody>
              <tbody v-if="rank.rankReportData && !loading['LOADING_RANK']">
              <tr v-for="(rank, index) in rank.rankReportData.data">
                <th scope="row">{{ rank.rank }}</th>
                <td style="overflow: scroll">
                  <div>
                    <img :src="rank.element.thumb_url" height="300px" alt="rank.element.title">

                    <a v-if="rank.element.type === 'video'" :href="rank.element.source_url" target="_blank">
                      <p>{{ rank.element.title }}</p>
                    </a>
                    <p v-if="rank.element.type === 'image'">{{ rank.element.title }}</p>
                  </div>
                </td>
                <td>{{ rank.final_win_rate | percent }}</td>
                <td>{{ rank.win_rate | percent }}</td>
              </tr>
              </tbody>
            </table>

            <div class="row">
              <div class="col-12" v-if="rank.rankReportData">
                <b-pagination
                  v-model="rank.currentPage"
                  :total-rows="rank.rankReportData.meta.total"
                  :per-page="rank.rankReportData.meta.per_page"
                  first-number
                  last-number
                  @change="handleRankPageChange"
                  align="center"
                ></b-pagination>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>


  </div>
</template>

<script>
import bsCustomFileInput from 'bs-custom-file-input';
import moment from 'moment';

// import { localize } from 'vee-validate';
export default {
  mounted() {
    bsCustomFileInput.init();
    this.loadPost();
    this.loadElements();
    this.loadRankReport();

    // localize('zh_TW');
    //debug
    // this.serial = 'a14b32';
  },
  props: {
    showPostEndpoint: String,
    playGameRoute: String,
    getElementsEndpoint: String,
    getRankEndpoint: String,
    updatePostEndpoint: String,
    updateElementEndpoint: String,
    deleteElementEndpoint: String,
    createImageElementEndpoint: String,
    createImageUrlElementEndpoint: String,
    createVideoElementEndpoint: String
  },
  data: function () {
    return {
      loading: {
        LOADING_POST: true,
        SAVING_POST: false,
        UPLOADING_VIDEO: false,
        LOADING_RANK: false,
        UPLOADING_IMAGE: false
      },
      uploadingFiles: {},
      post: null,
      isEditing: false,
      postElements: [],
      elements: [],
      stashElement: [],
      playingVideo: null,
      deletingElement: [],

      currentPage: 1,
      perPage: 50,
      videoUrl: "",
      imageUrl: "",

      // Alert
      dismissSecs: 5,
      dismissCountDown: 0,
      alertLevel: 'success',
      alertText: '',

      // search elements
      filters: {
        title_like: null
      },

      //rank
      rank: {}

    }
  },
  computed: {
    totalRow: function () {
      return this.elements.length;
    },
    filteredElements: function () {
      return _.filter(this.elements, (element) => {
        if (this.filters.title_like) {
          return element.title.includes(this.filters.title_like.toUpperCase())
            || element.title.includes(this.filters.title_like.toLowerCase());
        }
        return true;
      });
    },

  },
  methods: {

    /** Alert **/
    showAlert(text, level = 'success') {
      this.alertText = text;
      this.alertLevel = level;
      this.dismissCountDown = this.dismissSecs;
    },
    dismissAlert() {
      this.dismissCountDown = 0;
    },

    /** Loading **/
    uploadLoadingStatus(key, status) {
      this.$set(this.loading, key, status);
    },

    /** Post **/
    loadPost: function () {
      this.uploadLoadingStatus('LOADING_POST', true);
      const url = this.showPostEndpoint;
      axios.get(url)
        .then(res => {
          this.post = res.data.data;
        })
        .finally(() => {
          this.uploadLoadingStatus('LOADING_POST', false);
        });

    },
    clickEdit: function () {
      this.isEditing = true;
    },
    savePost: function () {
      this.uploadLoadingStatus('SAVING_POST', true);
      const data = {
        title: this.post.title,
        description: this.post.description,
        policy: {
          access_policy: this.post.policy
        }
      };

      axios.put(this.updatePostEndpoint, data)
        .then(res => {
          this.post = res.data.data;
          this.isEditing = false;
        })
        .finally(() => {
          this.uploadLoadingStatus('SAVING_POST', false);
        });
    },

    /** Elements **/
    loadElements: function (page = 1) {
      const params = {
        page: page
      };
      axios.get(this.getElementsEndpoint, {params: params})
        .then(res => {
          this.postElements = res.data;
          let counter = 1;
          _.each(this.postElements.data, (element) => {
            setTimeout(() => {
              this.elements.push(element);
            }, 100 * counter++);
          });

          if (this.postElements.current_page < this.postElements.last_page) {
            this.loadElements(this.postElements.current_page + 1);
          }
        })
    },
    isElementInPage: function (index) {
      return (this.currentPage - 1) * this.perPage <= index
        && this.currentPage * this.perPage > index;
    },
    updateElementTitle: function (id, event) {
      const data = {
        title: event.target.value
      };
      const url = this.updateElementEndpoint.replace('_id', id);
      axios.put(url, data)
        .then((res) => {

        })
    },
    deleteElement: function (element) {
      this.pushDeleting(element);

      const url = this.deleteElementEndpoint.replace('_id', element.id);
      axios.delete(url)
        .then((res) => {
          this.stashElement.push(element);
          const index = _.findIndex(this.elements, {
            id: element.id
          });
          this.$delete(this.elements, index);
        })
        .finally(() => {
          this.removeDeleting(element);
        });
    },
    pushDeleting(element) {
      this.deletingElement.push(element.id);
    },
    removeDeleting(element) {
      _.remove(this.deletingElement, (v) => {
        return v === element.id;
      });
    },
    isDeleting: function (element) {
      return this.deletingElement.includes(element.id);
    },

    /** Image **/
    uploadImages: function (event) {
      Array.from(event.target.files).forEach(file => {
        let form = new FormData();
        form.append('file', file);
        form.append('post_serial', this.post.serial);
        axios.post(this.createImageElementEndpoint, form, {
          onUploadProgress: progressEvent => {
            this.updateProgressBarValue(file, progressEvent);
          }
        })
          .then(res => {
            this.elements.push(res.data.data);
            this.deleteProgressBarValue(file);
          })
          .catch((err) => {
            this.showAlert(err.response.data.message, 'danger');
          });
      });
    },
    uploadImageUrl: function () {
      if (this.loading['UPLOADING_IMAGE']) {
        return;
      }
      this.uploadLoadingStatus('UPLOADING_IMAGE', true);
      const data = {
        post_serial: this.post.serial,
        url: this.imageUrl
      };
      this.imageUrl = "";
      axios.post(this.createImageUrlElementEndpoint, data)
        .then(res => {
          this.elements.push(res.data.data);
          this.showAlert(res.data.data.title);
        })
        .catch((err) => {
          this.showAlert(err.response.data.message, 'danger');
        })
        .finally(() => {
          this.uploadLoadingStatus('UPLOADING_IMAGE', false);
        });
    },

    /** Video **/
    uploadVideo: function () {
      if (this.loading['UPLOADING_VIDEO']) {
        return;
      }
      this.uploadLoadingStatus('UPLOADING_VIDEO', true);
      const data = {
        post_serial: this.post.serial,
        url: this.videoUrl
      };
      this.videoUrl = "";
      axios.post(this.createVideoElementEndpoint, data)
        .then(res => {
          this.elements.push(res.data.data);
          this.showAlert(res.data.data.title);
        })
        .catch((err) => {
          this.showAlert(err.response.data.message, 'danger');
        })
        .finally(() => {
          this.uploadLoadingStatus('UPLOADING_VIDEO', false);
        });

    },
    updateVideoScope: function (index, element, event) {
      let key = event.target.name;
      let seconds = this.toSeconds(event.target.value);
      if (!Number.isInteger(seconds)) {
        return;
      }
      const data = {
        [key]: seconds
      };
      const url = this.updateElementEndpoint.replace('_id', element.id);
      axios.put(url, data)
        .then((res) => {
          element[key] = seconds;
          this.$set(this.elements, index, element);
        })
    },
    updateProgressBarValue: function (file, progressEvent) {
      let filename = file.name;
      this.$set(this.uploadingFiles, filename, Math.round(progressEvent.loaded / progressEvent.total * 100));
    },
    deleteProgressBarValue: function (file) {
      delete this.uploadingFiles[file.name];
      this.uploadingFiles = Object.assign({}, this.uploadingFiles);
    },
    toTimeFormat: function (seconds) {
      if (seconds === null || seconds === '') {
        return null;
      }
      if (seconds >= 3600) {
        return moment.utc(seconds * 1000).format('HH:mm:ss');
      } else {
        return moment.utc(seconds * 1000).format('mm:ss');
      }
    },
    toSeconds: function (time) {
      let timeGroup = time.match(/^(?:(?:([01]?\d|2[0-3]):)?([0-5]?\d):)?([0-5]?\d)$/);
      return moment.duration({
        seconds: timeGroup[3],
        minutes: timeGroup[2],
        hours: timeGroup[1],
      }).asSeconds();
    },

    /** Player **/
    getPlayer(element) {
      return _.get(this.$refs, element.id + '.0.player', null);
    },
    clickPlayButton(index, element) {
      this.playingVideo = element.id;
      element.loadedVideo = true;
      this.$set(this.elements, index, element);

      this.doPlay(element);
    },
    doPlay(element) {
      const player = this.getPlayer(element);
      if (player) {
        window.player = player;
        player.loadVideoById({
          videoId: element.video_id,
          startSeconds: element.video_start_second,
          endSeconds: element.video_end_second
        });
      }
    },

    /** Rank **/
    loadRankReport: function (page = 1) {
      this.uploadLoadingStatus('LOADING_RANK', true);
      const filter = {
        'page': page
      };
      axios.get(this.getRankEndpoint, {params: filter})
        .then(res => {
          this.$set(this.rank, 'rankReportData', res.data);
          this.$set(this.rank, 'currentPage', res.data.meta.current_page);
        })
        .finally(() => {
          this.uploadLoadingStatus('LOADING_RANK', false);
        });
    },
    handleRankPageChange: function (page) {
      this.loadRankReport(page);
    }

  }
}

</script>

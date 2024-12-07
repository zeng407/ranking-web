<template>
  <!--  Main -->
  <div class="container-fluid">

    <!-- Alert -->
    <div class="position-fixed" style="top: 20px; right: 15px; z-index: 10">
      <span class="cursor-pointer" @click="dismissAlert">
        <b-alert :show="dismissCountDown" @dismissed="dismissCountDown = 0" dismissible fade :variant="alertLevel">
          {{ alertText }}
        </b-alert>
      </span>
    </div>

    <div class="row">

      <!-- tabs -->
      <div class="col-12 mt-2" style="z-index: 0;">
        <div class="nav flex-row nav-pills bg-secondary" id="v-pills-tab" role="tablist" aria-orientation="vertical">
          <a class="nav-link active text-white nav-link-hover" id="v-pills-post-info-tab" data-toggle="pill" href="#v-pills-post-info" role="tab"
            aria-controls="v-pills-post-info" aria-selected="true">
            <i v-if="isEditing" class="fa-xl fa-solid fa-triangle-exclamation" style="color:red" v-b-tooltip.hover
              :title="$t('Unsaved')"></i>
            <span class="d-inline-block text-center" style="width: 30px">
              <i class="fas fa-info-circle"></i>
            </span>
            {{ $t('edit_post.tab.info') }}
          </a>
          <a class="nav-link text-white nav-link-hover" id="v-pills-elements-tab" data-toggle="pill" href="#v-pills-elements" role="tab"
            aria-controls="v-pills-elements" aria-selected="false">
            <span class="d-inline-block text-center" style="width: 30px">
              <i class="fas fa-photo-video"></i>
            </span>
            {{ $t('edit_post.tab.element') }}
          </a>
          <a class="nav-link text-white nav-link-hover" id="v-pills-rank-tab" data-toggle="pill" href="#v-pills-rank" role="tab"
            aria-controls="v-pills-rank" aria-selected="false"
            @click="loadRankIframe">
            <span class="d-inline-block text-center" style="width: 30px">
              <i class="fas fa-trophy"></i>
            </span>
            {{ $t('edit_post.tab.rank') }}
          </a>
        </div>
      </div>

      <div class="col-12">
        <div class="tab-content" id="v-pills-tabContent">

          <!-- tab info -->
          <div class="tab-pane fade show active" id="v-pills-post-info" role="tabpanel"
            aria-labelledby="v-pills-post-info-tab">
            <!-- Loading -->
            <div class="col-9 fa-3x text-center" v-if="loading['LOADING_POST']">
              <i class="fas fa-spinner fa-spin"></i>
            </div>

            <ValidationObserver v-slot="{ invalid }" v-if="post && !loading['LOADING_POST']">
              <div class="row">
                <div class="col-6">
                  <h2 class="mt-3 mb-3">{{ $t('edit_post.info.head') }}</h2>
                </div>
                <div class="col-6">
                  <h2 class="mt-3 mb-3">
                    <!-- buttons -->
                    <span class="d-flex justify-content-end">
                      <button v-if="isShareable && !isEditing" id="share-button" type="button" class="btn btn-outline-dark mr-3"
                        @click="share">
                        {{$t('Share')}} &nbsp;<i class="fas fa-share-square"></i>
                      </button>
                      <b-popover target="share-button" placement="bottom" :show.sync="showSharePopover">
                        {{$t('Copied link')}}
                      </b-popover>

                      <span v-if="!isEditing"
                        id="play-button" class="btn btn-outline-dark mr-3"
                        :class="{disabled: elements.data.length < 2}"
                        @click="playGame" @mouseover="playButtonOnHoverIn" @mouseleave="playButtonOnHoverOut">
                        {{ $t('edit_post.info.start') }}&nbsp;<i class="fas fa-play"></i>
                      </span>
                      <b-popover :show.sync="showPlayPopover" target="play-button" placement="bottom">
                        {{ $t('edit_post.info.play_game_hint') }}
                      </b-popover>

                      <button class="btn btn-outline-dark" v-if="!isEditing" @click="clickEdit">
                        {{ $t('edit_post.info.edit') }}&nbsp;<i class="fas fa-edit"></i>
                      </button>

                      <!-- Editing -->
                      <button class="btn btn-secondary mr-3" v-if="isEditing" @click="cancelEdit"
                        :disabled="loading['SAVING_POST']">
                        {{ $t('edit_post.info.cancel_save') }}&nbsp;
                        <i class="fa-solid fa-rectangle-xmark"></i>
                      </button>
                      <button class="btn btn-primary" v-if="isEditing" @click="savePost"
                        :disabled="invalid || loading['SAVING_POST']">
                        {{ $t('edit_post.info.save') }}&nbsp;
                        <i class="fas fa-save" v-if="!loading['SAVING_POST']"></i>
                        <i class="fas fa-spinner fa-spin" v-if="loading['SAVING_POST']"></i>
                      </button>
                    </span>
                  </h2>
                </div>
              </div>
              <form @submit.prevent>
                <!-- 標題 -->
                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg" for="title">
                        {{ $t('Title') }}
                      </label>
                      <ValidationProvider :name="$t('Title')" rules="required" v-slot="{ errors }">
                        <input type="text" class="form-control" id="title" v-model="post.title" required
                          autocomplete="off" :disabled="!isEditing || loading['SAVING_POST']"
                          :maxlength="config.post_title_size">
                        <span class="text-danger">{{ errors[0] }}</span>
                      </ValidationProvider>
                      <CountWords v-if="isEditing" :words="post.title" :maxLength="config.post_title_size"></CountWords>
                    </div>
                  </div>
                </div>
                <!-- 描述 -->
                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg" for="description">
                        {{ $t('Description') }}
                      </label>
                      <ValidationProvider :name="$t('Description')" rules="required" v-slot="{ errors }">
                        <textarea class="form-control" id="description" v-model="post.description" rows="3"
                          style="resize: none" :maxlength="config.post_description_size"
                          aria-describedby="description-help" required :disabled="!isEditing || loading['SAVING_POST']">
                        </textarea>
                        <small id="description-help" class="form-text text-muted">
                          {{ $t('create_game.description.hint') }}
                        </small>
                        <CountWords v-if="isEditing" :words="post.description" :maxLength="config.post_description_size"></CountWords>
                        <span class="text-danger">{{ errors[0] }}</span>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>
                <!-- 標籤 -->
                <div class="row">
                  <div class="col-6">
                    <div class="form-group">
                      <label class="col-form-label-lg">
                        {{ $t('Tags') }}
                      </label>
                      <ValidationProvider>
                        <div class="input-group mb-3" v-if="isEditing">
                          <div class="input-group-prepend">
                            <span class="input-group-text" id="hashtag">#</span>
                          </div>
                          <input list="tagsOptions" type="text" class="form-control" autocomplete="off"
                            :placeholder="$t('edit_post.info.max_hashtag')" maxlength="15" aria-label="hashtag"
                            aria-describedby="hashtag" v-model="tagInput" @keyup="loadTagsOptions">
                          <datalist id="tagsOptions">
                            <option v-for="(tag) in tagsOptions" :value="tag.name" :key="tag.name" >{{ tag.name }}</option>
                          </datalist>
                          <div class="input-group-append">
                            <button class="btn btn-outline-secondary" type="button" id="hashtag" @click="addTag"
                              :disabled="tags.length >= 5">{{ $t('edit_post.tag.enter') }}</button>
                          </div>
                        </div>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>
                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <span v-for="tag in tags" :key="tag" class="badge badge-secondary mr-1" style="font-size: larger;" >
                        {{ tag }}
                        <a v-if="isEditing" class="btn btn-sm btn-light ml-1" @click="removeTag(tag)"
                          @keydown.enter.prevent>
                          <i class="fas fa-times"></i>
                        </a>
                      </span>
                      <span v-if="tags.length === 0" class="text-muted">{{ $t('edit_post.info.no_tag') }}</span>
                    </div>
                  </div>
                </div>
                <!-- 發佈 -->
                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg">
                        {{ $t('Publishment') }}
                      </label>
                      <div v-if="isEditing">
                        <ValidationProvider rules="required" v-slot="{ errors }">
                          <label class="form-control btn btn-outline-dark" for="post-privacy-public">
                            <input type="radio" id="post-privacy-public" v-model="post.policy" value="public"
                              :disabled="loading['SAVING_POST']">
                            {{ $t('post_policy.public') }}
                          </label>

                          <label class="form-control btn btn-outline-dark" for="post-privacy-private">
                            <input type="radio" id="post-privacy-private" v-model="post.policy" value="private"
                              :disabled="loading['SAVING_POST']">
                            {{ $t('post_policy.private') }}
                          </label>

                          <label class="form-control btn btn-outline-dark" for="post-privacy-password">
                            <input type="radio" id="post-privacy-password" v-model="post.policy" value="password"
                              :disabled="loading['SAVING_POST']">
                            {{ $t('post_policy.password') }}
                          </label>
                          <span class="text-danger">{{ errors[0] }}</span>
                        </ValidationProvider>
                        <div v-if="post.policy === 'password'">
                          <label class="col-form-label-lg" for="post-password">
                            {{ $t('edit_post.new_password') }}
                            <input id="post-password" type="text" class="form-control" v-model="post.password" maxlength="255">
                          </label>
                        </div>
                      </div>
                      <div v-else>
                        <input class="form-control" disabled="disabled" :value="$t('post_policy.' + post.policy)">
                        <small class="form-text text-muted" v-if="post.policy === 'public'">{{$t('edit_post.at_least_element_number_hint') }}</small>
                      </div>
                    </div>
                  </div>
                </div>
              </form>

              <div class="row" v-if="post">
                <!-- 建立時間 -->
                <div class="col-md-3 col-sm-6">
                  <div class="form-group">
                    <label class="col-form-label-lg">{{ $t('edit_post.info.create_time') }}</label>
                    <input class="form-control" disabled :value="post.created_at | date">
                  </div>
                </div>
                <!-- 遊戲次數 -->
                <div class="col-md-3 col-sm-6">
                  <div class="form-group">
                    <label class="col-form-label-lg">{{ $t('edit_post.rank.game_plays') }}</label>
                    <div class="">
                      <span class="pr-2 badge badge-secondary">
                        <h6 class="m-1">
                        {{ $t('my_games.table.played_all') }}&nbsp;<i class="fas fa-play-circle"></i>&nbsp;{{ post.play_count }}
                        </h6>
                      </span>
                      <span class="pr-2 badge badge-secondary">
                        <h6 class="m-1">
                        {{ $t('my_games.table.played_last_week') }}&nbsp;<i class="fas fa-play-circle"></i>&nbsp;{{ post.last_week_play_count }}
                        </h6>
                      </span>
                      <span class="pr-2 badge badge-secondary">
                        <h6 class="m-1">
                        {{ $t('my_games.table.played_this_week') }}&nbsp;<i class="fas fa-play-circle"></i>&nbsp;{{ post.this_week_play_count }}
                        </h6>
                      </span>
                    </div>
                  </div>
                </div>
              </div>
              <!-- Delete Post Button -->
              <div class="row" v-if="post && isEditing">
                <div class="col-12">
                  <div class="form-group fa-pull-right">
                    <button class="btn btn-danger" @click="deletePost" :disabled="loading['DELETING_POST']">
                      <i class="fas fa-trash-alt"></i> {{ $t('edit_post.info.delete') }}
                    </button>
                  </div>
                </div>
              </div>
            </ValidationObserver>
          </div>

          <!-- tab elements -->
          <div class="tab-pane fade" id="v-pills-elements" role="tabpanel" aria-labelledby="v-pills-elements-tab">
            <!-- upload image from device -->
            <!-- 上傳圖片/影片 -->
            <h2 class="mt-3 mb-3"><i class="fa-solid fa-upload"></i>&nbsp;{{ $t('edit_post.upload_media') }}</h2>

            <div class="row">
              <div class="col-12">
                <label for="image-upload">&nbsp;{{ $t('upload_from_local', {limit: config.upload_media_file_size_mb, rate_limit: config.upload_media_size_mb_at_a_time, rate_count: config.upload_media_file_count_at_a_time}) }}</label>
                <div class="custom-file form-group">
                  <input type="file" accept="image/*,video/*,audio/*" class="custom-file-input" id="image-upload" multiple @change="uploadMedias">
                  <label class="custom-file-label" for="image-upload">{{$t('Choose File...')}}</label>
                </div>
              </div>
            </div>
            <!-- upload progress bar -->
            <div class="progress mb-1" v-for="(progress, name) in uploadingFiles">
              <div class="progress-bar progress-bar-striped progress-bar-animated" role="progressbar" aria-valuemin="0"
                aria-valuemax="100" :aria-valuenow="progress" :style="{ 'width': progress + '%' , 'background' : progress == -1 ? 'red': ''}">
                <!-- if progress == -1 , add dismiss button -->
                <span v-if="progress != -1">{{ name }}</span>
                <span v-if="progress == -1" class="cursor-pointer" @click="cancelUpload(name)">
                  {{ name }}
                  <i class="fas fa-times"></i>
                </span>
              </div>
            </div>

            <!-- batch upload -->
            <!-- 從網址上傳 -->
            <h2 class="mt-5 mb-3"><i class="fa-solid fa-link"></i>&nbsp;{{ $t('edit_post.upload_batch') }}</h2>
            <div class="row mt-3">
              <div class="col-12">
                <label>&nbsp;{{ $t('Upload from URL') }}</label>
                <div class="input-group">
                  <textarea class="form-control" type="text" id="batchCreate" name="batchCreate" rows="5"
                    v-model="batchString" aria-describedby="batchCreateVideo"></textarea>
                  <div class="input-group-append">
                    <button class="btn btn-outline-secondary" type="button" @click="batchUpload"
                      :disabled="loading['BATCH_UPLOADING']">
                      <span v-show="!loading['BATCH_UPLOADING']">{{ $t('edit_post.add_video_button') }}</span>
                      <i v-show="loading['BATCH_UPLOADING']" class="fas fa-spinner fa-spin"></i>
                    </button>
                  </div>
                </div>
                <div class="mt-3">
                    <UploadGuide />
                </div>
              </div>
            </div>

            <!-- search -->
            <h2 class="mt-5 mb-3"><i class="fa-solid fa-photo-film"></i>&nbsp;{{ $t('edit_post.edit_media') }}</h2>
            <p>{{ $t('Max :number elements',{ number: config.post_max_element_count }) }}</p>

            <nav class="navbar navbar-light bg-light pr-0 pl-0 justify-content-end">
              <div class="form-inline mr-auto p-0 col-auto">
                <h5 class="mr-1">
                  <span class="badge badge-secondary cursor-pointer" @click="sortByTitle">{{ $t('edit_post.sort_by_title') }}
                  <i v-if="sorter.sort_by == 'title' && sorter.sort_dir == 'asc'" class="fa-solid fa-sort-down"></i>
                  <i v-else-if="sorter.sort_by == 'title' && sorter.sort_dir == 'desc'" class="fa-solid fa-sort-up"></i>
                  <i v-else class="fa-solid fa-sort"></i>
                  </span>
                </h5>
                <h5 class="mr-1">
                  <span class="badge badge-secondary cursor-pointer" @click="sortByRank">{{ $t('edit_post.sort_by_rank') }}
                  <i v-if="sorter.sort_by == 'rank' && sorter.sort_dir == 'asc'" class="fa-solid fa-sort-down"></i>
                  <i v-else-if="sorter.sort_by == 'rank' && sorter.sort_dir == 'desc'" class="fa-solid fa-sort-up"></i>
                  <i v-else class="fa-solid fa-sort"></i>
                  </span>
                </h5>
                <h5 class="mr-1">
                  <span class="badge badge-secondary cursor-pointer" @click="sortById">{{ $t('edit_post.sort_by_created_time') }}
                  <i v-if="sorter.sort_by == 'id' && sorter.sort_dir == 'asc'" class="fa-solid fa-sort-down"></i>
                  <i v-else-if="sorter.sort_by == 'id' && sorter.sort_dir == 'desc'" class="fa-solid fa-sort-up"></i>
                  <i v-else class="fa-solid fa-sort"></i>
                  </span>
                </h5>
              </div>
              <div class="form-inline p-0 col-md-auto col-sm-12">
                <input class="form-control mr-sm-2 " v-model="filters.title_like" type="search"
                  :placeholder="$t('Search')" aria-label="Search" @change="loadElements(1)">
                  <span class="ml-1 btn-sm btn btn-light" @click="resetSearch"><i class="fas fa-xmark-circle"></i></span>
                  <span class="btn-sm btn btn-light"><i class="fa-solid fa-magnifying-glass"></i></span>
              </div>
            </nav>
            <p>{{ $t('total elements', { count: totalRow }) }}</p>

            <!-- display type -->
            <div class="d-flex justify-content-end my-4">
              <div class="btn-group btn-group-toggle" data-toggle="buttons">
                <label class="btn btn-outline-dark">
                  <input type="radio" v-model="displayType" value="card">
                  <i class="fas fa-th"></i>
                </label>
                <label class="btn btn-outline-dark">
                  <input type="radio" v-model="displayType" value="list">
                  <i class="fas fa-list"></i>
                </label>
              </div>
            </div>

            <!-- elements -->
            <div class="row" v-if="displayType === 'card'">
              <!-- display by card -->
              <template v-for="(element, index) in elements.data">
                <!-- show video card -->
                <div class="col-lg-4 col-md-6" v-if="isVideoSource(element)" :key="element.id + '_' + index+'_'+element.source_url">
                  <!-- youtube source -->
                  <div class="card mb-3" v-if="isYoutubeSource(element)">
                    <youtube v-if="isYoutubeSource(element) && element.loadedVideo" width="100%" height="270"
                      :ref="element.id" @ready="doPlay(element)" :player-vars="{
                      controls: 1,
                      autoplay: 0,
                      start: element.video_start_second,
                      end: element.video_end_second,
                      rel: 0,
                      origin: origin
                    }"></youtube>
                    <img :src="element.thumb_url" class="card-img-top" :alt="element.title"
                      v-show="isYoutubeSource(element) && !element.loadedVideo">
                    <!-- youtube video editor -->
                    <div class="card-body">
                      <!--title edit-->
                      <textarea class="form-control-plaintext bg-light cursor-pointer p-2 mb-2" v-model="element.title"
                        :maxlength="config.element_title_size" rows="4" style="resize: none;"
                        @change="updateElementTitle(element.id, $event)"></textarea>
                      <!--play time range-->
                      <div class="row mb-3">
                        <div class="col-10">
                          <div class="input-group">
                            <div class="input-group-prepend d-lg-none d-xl-block">
                              <span class="input-group-text">{{ $t('edit_post.video_range') }}</span>
                            </div>
                            <input type="text" class="form-control" name="video_start_second" placeholder="0:00"
                              aria-label="start" @change="updateVideoScope(index, element, $event)"
                              :value="toTimeFormat(element.video_start_second)">
                            <div class="input-group-prepend"><span class="input-group-text">~</span></div>
                            <input type="text" class="form-control" name="video_end_second"
                              :placeholder="toTimeFormat(element.video_duration_second)" aria-label="end"
                              :value="toTimeFormat(element.video_end_second)"
                              @change="updateVideoScope(index, element, $event)">
                          </div>
                        </div>
                        <!--play button-->
                        <div class="col-2">
                          <a class="btn btn-danger fa-pull-right" @click="clickYoutubePlayButton(index, element)">
                            <i class="fas fa-play-circle"></i>
                          </a>
                        </div>
                      </div>
                      <div>
                        <!--rank-->
                        <span class="card-text d-inline-block">
                          <small class="text-muted">{{ $t('edit_post.rank')}} # {{ getElementRank(element) }}</small>
                          <!--create time-->
                          <br>
                          <small class="text-muted">{{ element.created_at | datetime}}</small>
                        </span>
                        <!--delete button-->
                        <a class="btn btn-danger fa-pull-right" @click="deleteElement(element)">
                          <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                          <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                        </a>
                        <!--edit button-->
                        <EditElement v-if="post"
                          :post-serial="post.serial"
                          :element-id="String(element.id)"
                          :source-url="element.source_url"
                          :update-element-route="updateElementEndpoint"
                          :upload-element-route="uploadElementEndpoint"
                          @elementUpdated="handleElementUpdated"/>
                      </div>
                    </div>
                  </div>

                  <!-- youtube embed source -->
                  <div class="card mb-3" v-else-if="isYoutubeEmbedSource(element)">
                    <YoutubeEmbed :element="element" v-if="element" :autoplay="false"/>
                    <!-- youtube video editor -->
                    <div class="card-body">
                      <!--title edit-->
                      <textarea class="form-control-plaintext bg-light cursor-pointer p-2 mb-2" v-model="element.title"
                        :maxlength="config.element_title_size" rows="4" style="resize: none;"
                        @change="updateElementTitle(element.id, $event)"></textarea>
                      <!--rank -->
                      <span class="card-text d-inline-block">
                        <small class="text-muted">{{ $t('edit_post.rank')}} # {{ getElementRank(element) }}</small>
                        <!--create time-->
                        <br>
                        <small class="text-muted">{{ element.created_at | datetime}}</small>
                      </span>

                      <!--delete button-->
                      <a class="btn btn-danger fa-pull-right" @click="deleteElement(element)">
                        <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                        <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                      </a>
                      <!--edit button-->
                      <EditElement v-if="post"
                        :post-serial="post.serial"
                        :element-id="String(element.id)"
                        :source-url="element.source_url"
                        :update-element-route="updateElementEndpoint"
                        :upload-element-route="uploadElementEndpoint"
                        @elementUpdated="handleElementUpdated"/>
                    </div>
                  </div>

                  <!-- twitch video -->
                  <div v-else-if="isTwitchVideoSource(element) || isTwitchClipSource(element)">
                    <div :id="'twitch-video-'+element.id" v-if="isTwitchVideoSource(element)" class="w-100 twitch-container"></div>
                    <iframe v-else-if="isTwitchClipSource(element) && element.loadedVideo"
                      :src="'https://clips.twitch.tv/embed?clip='+element.video_id+'&parent='+host+'&autoplay=true'"
                      height="270"
                      width="100%"
                      allowfullscreen></iframe>
                    <img v-if="!element.loadedVideo" :src="element.thumb_url" class="card-img-top" :alt="element.title">
                      <!-- editor -->
                      <div class="card-body">
                      <!--title edit-->
                      <textarea class="form-control-plaintext bg-light cursor-pointer p-2 mb-2" v-model="element.title"
                        :maxlength="config.element_title_size" rows="4" style="resize: none;"
                        @change="updateElementTitle(element.id, $event)"></textarea>
                      <!--play time range-->
                      <div class="row mb-3">
                        <div class="col-10">
                          <div class="input-group">
                            <div class="input-group-prepend d-lg-none d-xl-block">
                              <span class="input-group-text">{{ $t('edit_post.video_range') }}</span>
                            </div>
                            <input type="text" class="form-control" name="video_start_second" placeholder="0:00"
                              aria-label="start" @change="updateVideoScope(index, element, $event)"
                              :value="toTimeFormat(element.video_start_second)">
                            <div class="input-group-prepend"><span class="input-group-text">~</span></div>
                            <input type="text" class="form-control" disabled>
                          </div>
                        </div>
                        <!--play button-->
                        <div class="col-2">
                          <a class="btn btn-danger fa-pull-right" @click="clickTwitchPlayButton(index, element)">
                            <i class="fas fa-play-circle"></i>
                          </a>
                        </div>
                      </div>
                      <div>
                        <!--rank-->
                        <span class="card-text d-inline-block">
                          <small class="text-muted">{{ $t('edit_post.rank')}} # {{ getElementRank(element) }}</small>
                          <!--create time-->
                          <br>
                          <small class="text-muted">{{ element.created_at | datetime}}</small>
                        </span>
                        <!--delete button-->
                        <a class="btn btn-danger fa-pull-right" @click="deleteElement(element)">
                          <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                          <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                        </a>
                        <!--edit button-->
                        <EditElement v-if="post"
                          :post-serial="post.serial"
                          :element-id="String(element.id)"
                          :source-url="element.source_url"
                          :update-element-route="updateElementEndpoint"
                          :upload-element-route="uploadElementEndpoint"
                          @elementUpdated="handleElementUpdated"/>
                      </div>
                    </div>
                  </div>

                  <!-- bilibili video -->
                  <div class="card mb-3" v-else-if="isBilibiliVideoSource(element)">
                    <BilibiliVideo :element="element" v-if="element" :autoplay="false" :muted="false" :preview="true"/>
                    <!-- video editor -->
                    <div class="card-body">
                      <!--title edit-->
                      <textarea class="form-control-plaintext bg-light cursor-pointer p-2 mb-2" v-model="element.title"
                        :maxlength="config.element_title_size" rows="4" style="resize: none;"
                        @change="updateElementTitle(element.id, $event)"></textarea>
                      <!--rank -->
                      <span class="card-text d-inline-block">
                        <small class="text-muted">{{ $t('edit_post.rank')}} # {{ getElementRank(element) }}</small>
                        <!--create time-->
                        <br>
                        <small class="text-muted">{{ element.created_at | datetime}}</small>
                      </span>

                      <!--delete button-->
                      <a class="btn btn-danger fa-pull-right" @click="deleteElement(element)">
                        <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                        <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                      </a>
                      <!--edit button-->
                      <EditElement v-if="post"
                        :post-serial="post.serial"
                        :element-id="String(element.id)"
                        :source-url="element.source_url"
                        :update-element-route="updateElementEndpoint"
                        :upload-element-route="uploadElementEndpoint"
                        @elementUpdated="handleElementUpdated"/>
                    </div>
                  </div>

                  <!-- video source -->
                  <div class="card mb-3" v-else>
                    <!-- load the video player -->
                    <video width="100%" height="270" loop controls playsinline :src="element.source_url" :poster="element.thumb_url"></video>
                    <!-- editor -->
                    <div class="card-body">
                      <input class="form-control-plaintext bg-light cursor-pointer mb-2 p-2" type="text"
                        :value="element.title" :maxlength="config.element_title_size"
                        @change="updateElementTitle(element.id, $event)">
                      <span class="card-text">
                        <small class="text-muted">{{ $t('edit_post.rank')}} # {{ getElementRank(element) }}</small>
                        <!--create time-->
                        <br>
                        <small class="text-muted">{{ element.created_at | datetime}}</small>
                      </span>
                      <!--delete button-->
                      <a class="btn btn-danger fa-pull-right" @click="deleteElement(element)">
                        <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                        <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                      </a>
                      <!--edit button-->
                      <EditElement v-if="post"
                        :post-serial="post.serial"
                        :element-id="String(element.id)"
                        :source-url="element.source_url"
                        :update-element-route="updateElementEndpoint"
                        :upload-element-route="uploadElementEndpoint"
                        @elementUpdated="handleElementUpdated"/>
                    </div>
                  </div>
                </div>

                <!-- image -->
                <div class="col-lg-4 col-md-6" v-if="element.type === 'image'" :key="element.id + '_' + index+'_'+element.source_url">
                  <div class="card mb-3">
                    <img @error="onImageError(element, $event)" :src="getThumbnailUrl(element)" class="card-img-top" :alt="element.title"
                      style="max-height: 300px; object-fit: contain;"
                      v-if="element.type === 'image'">

                    <div class="card-body">
                      <!-- title -->
                      <textarea class="form-control-plaintext bg-light cursor-pointer p-2 mb-2" v-model="element.title"
                        :maxlength="config.element_title_size" rows="4" style="resize: none;"
                        @change="updateElementTitle(element.id, $event)"></textarea>
                      <span class="card-text">
                        <small class="text-muted">{{ $t('edit_post.rank')}} # {{ getElementRank(element) }}</small>
                        <!--create time-->
                        <br>
                        <small class="text-muted">{{ element.created_at | datetime}}</small>
                      </span>
                      <!-- delete button -->
                      <a class="btn btn-danger fa-pull-right" @click="deleteElement(element)">
                        <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                        <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                      </a>
                      <!--edit button-->
                      <EditElement v-if="post"
                        :post-serial="post.serial"
                        :element-id="String(element.id)"
                        :source-url="element.source_url"
                        :update-element-route="updateElementEndpoint"
                        :upload-element-route="uploadElementEndpoint"
                        @elementUpdated="handleElementUpdated"/>
                    </div>
                  </div>
                </div>
              </template>
            </div>

            <!-- display by table list -->
            <div v-else class="table-responsive">
              <table class="table table-bordered white-space-no-wrap">
                <thead>
                  <tr>
                    <th style="width: 200px;"></th>
                    <th style="width: 400px;">{{ $t('edit_post.element.title') }}</th>
                    <th>{{ $t('edit_post.element.type') }}</th>
                    <th>{{ $t('edit_post.element.rank') }}</th>
                    <th style="width: 100px;">{{ $t('edit_post.element.video_start') }}</th>
                    <th style="width: 100px;">{{ $t('edit_post.element.video_end') }}</th>
                    <th style="width: 100px;">{{ $t('edit_post.element.created_at') }}</th>
                    <th style="width: 100px;">{{ $t('edit_post.element.action') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(element, index) in elements.data">
                    <!-- image or video -->
                    <td>
                      <template v-if="isVideoSource(element)">
                        <!-- youtube source -->
                        <template v-if="isYoutubeSource(element)">
                          <youtube v-if="isYoutubeSource(element) && element.loadedVideo" width="200px" height="100px"
                            :ref="element.id" @ready="doPlay(element)" :player-vars="{
                            controls: 1,
                            autoplay: 0,
                            start: element.video_start_second,
                            end: element.video_end_second,
                            rel: 0,
                            origin: origin
                          }"></youtube>
                          <img :src="element.thumb_url" class="card-img-top" :alt="element.title"
                            v-show="isYoutubeSource(element) && !element.loadedVideo" style="width: 200px;">
                        </template>


                        <!-- youtube embed source -->
                        <template v-else-if="isYoutubeEmbedSource(element)">
                          <YoutubeEmbed :element="element" v-if="element" :autoplay="false" width="200px" height="100px"/>
                        </template>

                        <!-- twitch video -->
                        <template v-else-if="isTwitchVideoSource(element) || isTwitchClipSource(element)">
                          <div :id="'twitch-video-'+element.id" v-if="isTwitchVideoSource(element)" class="w-100 twitch-container"></div>
                          <iframe v-else-if="isTwitchClipSource(element) && element.loadedVideo"
                            :src="'https://clips.twitch.tv/embed?clip='+element.video_id+'&parent='+host+'&autoplay=true'"
                            width="200px" height="100px"
                            allowfullscreen></iframe>
                          <img v-if="!element.loadedVideo" :src="element.thumb_url" class="card-img-top" :alt="element.title">
                        </template>

                        <!-- bilibili video -->
                        <template v-else-if="isBilibiliVideoSource(element)">
                          <BilibiliVideo :element="element" v-if="element" :autoplay="false" :muted="false" :preview="true" width="200px" height="100px"/>
                        </template>

                        <!-- video source -->
                        <template v-else>
                          <video width="200px" height="100px" loop controls playsinline :src="element.source_url" :poster="element.thumb_url"></video>
                        </template>
                      </template>
                      <template v-else>
                        <img @error="onImageError(element, $event)" :src="getThumbnailUrl(element)" class="card-img-top" :alt="element.title"
                          v-if="element.type === 'image'" style="width: 200px; height: 100px; object-fit: contain;">
                      </template>
                    </td>
                    <!-- title -->
                    <td>
                      <textarea class="form-control-plaintext bg-light cursor-pointer p-2 mb-2" v-model="element.title"
                        :maxlength="config.element_title_size" rows="4" style="resize: none; width: 200px;"
                        @change="updateElementTitle(element.id, $event)"></textarea>
                    </td>
                    <!-- type -->
                    <td>{{ $t('element.type.'+element.type) }}</td>
                    <!-- rank -->
                    <td>{{ getElementRank(element) }}</td>
                    <!-- video start -->
                    <td>
                      <!-- only support for youtube, twitch -->
                      <template v-if="isYoutubeSource(element) || isTwitchVideoSource(element) || isTwitchClipSource(element)">
                        <input type="text" class="form-control" name="video_start_second" placeholder="0:00"
                          aria-label="start" @change="updateVideoScope(index, element, $event)"
                          :value="toTimeFormat(element.video_start_second)">
                      </template>
                    </td>
                    <!-- video end -->
                    <td>
                      <!-- only support for youtube-->
                      <template v-if="isYoutubeSource(element)">
                        <input type="text" class="form-control" name="video_end_second"
                          :placeholder="toTimeFormat(element.video_duration_second)" aria-label="end"
                          :value="toTimeFormat(element.video_end_second)"
                          @change="updateVideoScope(index, element, $event)">
                        </template>
                    </td>
                    <!-- create time -->
                    <td>{{ element.created_at | datetime}}</td>
                    <!-- action -->
                    <td>
                      <span class="m-1">
                        <a class="btn btn-danger fa-pull-right" @click="deleteElement(element)">
                          <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                          <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                        </a>
                      </span>
                      <span class="m-1">
                        <EditElement v-if="post"
                          :post-serial="post.serial"
                          :element-id="String(element.id)"
                          :source-url="element.source_url"
                          :update-element-route="updateElementEndpoint"
                          :upload-element-route="uploadElementEndpoint"
                          @elementUpdated="handleElementUpdated"/>
                      </span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>


            <!-- per page setting -->
            <div class="d-flex justify-content-end">
              <div class="form-group" v-if="elements.data.length > 0">
                <label class="col-form-label">{{ $t('edit_post.element.per_page') }}</label>
                <select class="form-control" v-model="elements.meta.per_page" @change="loadElements(1)">
                  <option v-for="option in [10,25,50,100]" :value="option">{{ option }}</option>
                </select>
              </div>
            </div>

            <!-- pagination -->
            <b-pagination v-model="currentPage" v-if="elements.meta.last_page > 1" :total-rows="totalRow"
              :per-page="elements.meta.per_page" first-number last-number @change="handleElementPageChange"
              align="center">
            </b-pagination>

            <div class="row" v-if="!elements.data || elements.data.length == 0">
              <div class="col-12">
                  <h5 class="text-center">
                    <div class="alert alert-secondary">
                      <i class="fa-solid fa-circle-exclamation"></i> {{ $t('edit_post.no_element') }}
                    </div>
                  </h5>
              </div>
            </div>
          </div>

          <!-- tab rank -->
          <div class="tab-pane fade" id="v-pills-rank" role="tabpanel" aria-labelledby="v-pills-rank-tab">
            <iframe id="rank-iframe" class="mt-2" width="100%" height="800px" frameborder="0"></iframe>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import bsCustomFileInput from 'bs-custom-file-input';
import moment from 'moment';
import Swal from 'sweetalert2';
import EditElement from './partials/EditElement.vue';
import UploadGuide from './partials/UploadGuide.vue';
import BilbiliVideo from './partials/BilibiliVideo.vue';

export default {
  mounted() {
    bsCustomFileInput.init();
    this.loadPost();
    this.resetSearch();
    this.origin = window.location.origin;
    this.host = window.location.host;
  },
  props: {
    config: Object,
    playGameRoute: String,
    gameRankRoute: String,
    showPostEndpoint: String,
    getElementsEndpoint: String,
    updatePostEndpoint: String,
    updateElementEndpoint: String,
    uploadElementEndpoint: String,
    deleteElementEndpoint: String,
    createImageElementEndpoint: String,
    batchCreateEndpoint: String,
    getTagsOptionsEndpoint: String
  },
  data: function () {
    return {
      host: '',
      origin: '',
      loading: {
        LOADING_POST: true,
        SAVING_POST: false,
        UPLOADING_YOUTUBE_VIDEO: false,
        LOADING_RANK: false,
        UPLOADING_IMAGE: false,
        UPLOADING_VIDEO_URL: false,
        BATCH_UPLOADING: false,
        DELETING_POST: false,
      },
      uploadingFiles: {},
      post: {
        title: '',
        description: '',
        policy: '',
        password: '',
        tags: [],

      },
      keep_post: null,
      isEditing: false,
      elements: {
        data: [],
        meta: {
          last_page: 1,
          per_page: 10
        }
      },
      playingVideo: null,
      deletingElement: [],

      currentPage: 1,
      youtubeUrl: "",
      videoUrl: "",
      batchString: "",
      imageUrl: "",

      // Alert
      dismissCountDown: 0,
      alertLevel: 'success',
      alertText: '',

      //hover
      showPlayPopover: false,
      showSharePopover: false,

      // search elements
      filters: {
        title_like: null
      },
      sorter: {
        sort_by: 'id',
        sort_dir: 'desc',
      },
      displayType: 'list',

      //rank
      rank: {},

      //tags
      tagInput: "",
      oldTagInput: "",
      tags: [],
      keep_tags: [],
      tagsOptions: [],
      tagLocalStash: {},

      //onImageError
      errorImages: [],
    }
  },
  components: {

  },
  computed: {
    totalRow: function () {
      if (this.elements && this.elements.meta) {
        return this.elements.meta.total;
      }
      return 0;
    },
    isShareable: function () {
      return this.elements.data.length > 1
      && (this.post.policy == 'public' || this.post.policy == 'password');
    },

  },
  methods: {

    /** Alert **/
    showAlert(text, level = 'success', dismissSecs = 10) {
      this.alertText = text;
      this.alertLevel = level;
      this.dismissCountDown = dismissSecs;
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
          this.tags = res.data.data.tags;
          this.keep_post = _.cloneDeep(res.data.data);
          this.keep_tags = _.cloneDeep(res.data.data.tags);
        })
        .finally(() => {
          this.uploadLoadingStatus('LOADING_POST', false);
        });
    },
    loadTwitchVideo: function (index, element) {
      // for each element, if it is twitch source, load the video
      if (element.twitchPlayer === undefined) {
        element.twitchPlayer = new Twitch.Embed("twitch-video-" + element.id, {
          width: "100%",
          height: 270,
          video: element.video_id,
          layout: "video",
          autoplay: true,
          muted: false,
          time: this.formatTime(element.video_start_second),
        });
        element.loadedVideo = true;
        this.$set(this.elements.data, index, element);
      }

      return element.twitchPlayer;
    },
    formatTime: function (time) {
      // format second to 0h0m0s
      let hour = Math.floor(time / 3600);
      let minute = Math.floor((time % 3600) / 60);
      let second = time % 60;
      return `${hour}h${minute}m${second}s`;
    },
    clickEdit: function () {
      this.isEditing = true;
    },
    cancelEdit: function () {
      Swal.fire({
        title: this.$t("Are you sure?"),
        text: this.$t("You will lose all unsaved changes!"),
        icon: "warning",
        confirmButtonText: this.$t("Yes"),
        cancelButtonText: this.$t("No"),
        showCancelButton: true,
      })
        .then((willDelete) => {
          if (willDelete.isConfirmed) {
            this.isEditing = false;
            this.post = _.cloneDeep(this.keep_post);
            this.tags = _.cloneDeep(this.keep_tags);
          }
        });
    },
    savePost: function () {
      this.uploadLoadingStatus('SAVING_POST', true);
      const data = {
        title: this.post.title,
        description: this.post.description,
        policy: {
          access_policy: this.post.policy,
          password: this.post.password
        },
        tags: this.tags
      };

      axios.put(this.updatePostEndpoint, data)
        .then(res => {
          this.post = res.data.data;
          this.tags = res.data.data.tags;
          this.keep_post = _.cloneDeep(res.data.data);
          this.keep_tags = _.cloneDeep(res.data.data.tags);
          this.isEditing = false;

          // show alert
          Swal.fire({
            position: "top-end",
            showConfirmButton: false,
            showCancelButton: true,
            cancelButtonText: this.$t("Close"),
            toast: true,
            title: this.$t("Updated!"),
            text: this.$t("Your post has been updated."),
            icon: "success",
            timer: 3000
          });
        })
        .catch(error => {
          // get first error
          let message = error.response.data.errors[Object.keys(error.response.data.errors)[0]];
          Swal.fire({
            position: "top-end",
            showConfirmButton: false,
            toast: true,
            title: this.$t("Error!"),
            text: message[0],
            icon: "error",
            timer: 3000
          });
        })
        .finally(() => {
          this.uploadLoadingStatus('SAVING_POST', false);
        });
    },
    deletePost: function () {
      this.uploadLoadingStatus('DELETING_POST', true);
      Swal.fire({
        title: this.$t('Enter Password'),
        input: 'password',
        inputAttributes: {
          autocapitalize: 'off'
        },
        showCancelButton: true,
        confirmButtonText: this.$t('Delete'),
        cancelButtonText: this.$t('Cancel'),
        showLoaderOnConfirm: true,
        preConfirm: (password) => {
          return axios.delete(this.updatePostEndpoint, { data: { password: password } })
            .then(res => {
            })
            .catch(error => {
              Swal.showValidationMessage(this.$t('Request failed'));
            });
        },
        allowOutsideClick: () => !Swal.isLoading()
      }).then((result) => {
        if (result.isConfirmed) {
          this.uploadLoadingStatus('LOADING_POST', true);
          Swal.fire({
            position: "top-end",
            showConfirmButton: false,
            toast: true,
            title: this.$t('Deleted!'),
            text: this.$t('Your post has been deleted.'),
            icon: 'success',
            timer: 3000
          }).then(res => {
            window.location.href = '/account/post';
          });
        }
      }).finally(() => {
        this.uploadLoadingStatus('DELETING_POST', false);
      });
    },

    /** Elements **/
    loadElements: function (page = 1) {
      const pagination = {
        page: page,
      };
      const params = {
        ...pagination,
        ...{ filter: this.filters },
        ...this.sorter,
        per_page: this.elements.meta.per_page
      }
      axios.get(this.getElementsEndpoint, { params: params })
        .then(res => {
          this.elements = res.data;
        })
        .finally(() => {
          // clear twitch player
          this.clearTwicthPlayer();
        });
    },
    clearTwicthPlayer: function () {
      this.elements.data.forEach((element) => {
        $(`#twitch-video-${element.id}`).empty();
      });
    },
    resetSearch: function () {
      this.filters.title_like = null;
      this.sorter = {
        'sort_by': 'id',
        'sort_dir': 'desc'
      };
      this.currentPage = 1;
      this.loadElements(1);
    },
    sortByTitle: function () {
      this.sorter = {
        'sort_by': 'title',
        'sort_dir': (this.sorter.sort_by === 'title' && this.sorter.sort_dir === 'asc') ? 'desc' : 'asc'
      };
      this.currentPage = 1;
      this.loadElements(1);
    },
    sortByRank: function () {
      this.sorter = {
        'sort_by': 'rank',
        'sort_dir': (this.sorter.sort_by === 'rank' && this.sorter.sort_dir === 'asc') ? 'desc' : 'asc'
      };
      this.currentPage = 1;
      this.loadElements(1);
    },
    sortById: function () {
      this.sorter = {
        'sort_by': 'id',
        'sort_dir': (this.sorter.sort_by === 'id' && this.sorter.sort_dir === 'asc') ? 'desc' : 'asc'
      };
      this.currentPage = 1;
      this.loadElements(1);
    },
    handleElementUpdated: function (data) {
      const index = _.findIndex(this.elements.data, {
        id: data.data.id
      });
      this.$set(this.elements.data, index, data.data);
      Swal.fire({
        position: "top-end",
        showConfirmButton: false,
        title: this.$t("Updated!"),
        toast: true,
        text: this.$t("The element has been updated."),
        icon: "success",
        timer: 3000
      });
    },
    updateElementTitle: function (id, event) {
      const data = {
        post_serial: this.post.serial,
        title: event.target.value
      };
      const url = this.updateElementEndpoint.replace('_id', id);
      axios.put(url, data)
        .then((res) => {
          Swal.fire({
            position: "top-end",
            showConfirmButton: false,
            title: this.$t("Updated!"),
            toast: true,
            text: this.$t("The element has been updated."),
            icon: "success",
            timer: 3000
          });
        })
    },
    deleteElement: function (element) {
      Swal.fire({
        title: this.$t("Are you sure?"),
        text: this.$t("Once deleted, you will not be able to recover this element!"),
        icon: "warning",
        showCancelButton: true,
      })
        .then((willDelete) => {
          if (willDelete.isConfirmed) {
            this.pushDeleting(element);

            const url = this.deleteElementEndpoint.replace('_id', element.id);
            axios.delete(url)
              .then((res) => {
                const index = _.findIndex(this.elements.data, {
                  id: element.id
                });
                this.$delete(this.elements.data, index);
                this.elements.meta.total--;
                Swal.fire({
                  position: "top-end",
                  title: this.$t("Deleted!"),
                  toast: true,
                  text: this.$t("The element has been deleted."),
                  showConfirmButton: false,
                  icon: "success",
                  timer: 3000
                });
                this.loadElements(this.currentPage);
              })
              .finally(() => {
                this.removeDeleting(element);
              });
          }
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

    /** Tag */
    addTag: function () {
      const tag = this.tagInput.trim().replace("#", "");
      if (tag.length > 20) {
        tag = tag.substring(0, 20);
      }
      if (this.tags.length >= 5) {
        Swal.fire({
          position: "top-end",
          showConfirmButton: false,
          title: this.$t("Maximum number of tags reached"),
          toast: true,
          icon: "warning",
          timer: 3000
        });
        return;
      }

      if (this.tags.includes(tag)) {
        Swal.fire({
          position: "top-end",
          showConfirmButton: false,
          title: this.$t("Hashtag already exists"),
          toast: true,
          icon: "warning",
          timer: 3000
        });
        return;
      }

      if (tag) {
        this.tags.push(tag);
        this.tagInput = "";
        this.loadTagsOptions();
      }
    },
    removeTag: function (tag) {
      _.remove(this.tags, (v) => {
        return v === tag;
      });
      this.tags = Object.assign([], this.tags);
    },
    loadTagsOptions: function () {
      if (this.tagInput === this.oldTagInput && this.tagInput !== "") {
        return;
      }

      if (this.tagLocalStash[this.tagInput]) {
        this.tagsOptions = this.tagLocalStash[this.tagInput];
        return;
      }

      if (this.tagInputTimeout) {
        clearTimeout(this.tagInputTimeout);
      }

      this.tagInputTimeout = setTimeout(() => {
        const params = {
          keyword: this.tagInput
        };

        axios.get(this.getTagsOptionsEndpoint, { params: params })
          .then(res => {
            this.tagsOptions = res.data;
            this.oldTagInput = this.tagInput;
            this.tagLocalStash[this.tagInput] = res.data;
            const tagLocalStashLength = Object.keys(this.tagLocalStash).length;
            if (tagLocalStashLength > 30) {
              delete this.tagLocalStash[Object.keys(this.tagLocalStash)[0]];
            }
          });
      }, 500);
    },

    /** Image/Video **/
    uploadMedias: function (event) {
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
            this.loadElements(1);
            this.deleteProgressBarValue(file);
            Swal.fire({
              position: "top-end",
              showConfirmButton: false,
              title: this.$t("Uploaded!"),
              toast: true,
              text: this.$t("The file has been uploaded."),
              icon: "success",
              timer: 3000
            });
          })
          .catch((err) => {
            this.showAlert(err.response.data.message || err.response.statusText, 'danger');
            this.setProgressBarValueFailed(file);
          });
      });
      event.target.value = '';
    },
    cancelUpload: function (file) {
      this.deleteProgressBarValue({ name: file });
    },
    onImageError: function (element, event) {
      if(this.errorImages.includes(element.id)) {
        return;
      }

      if(element.imgur_url !== null) {
        event.target.src = element.imgur_url;
      }
      this.errorImages.push(element.id);
    },

    /** Video **/
    isVideoSource: function (element) {
      return element.type === 'video';
    },
    isYoutubeSource: function (element) {
      return element.type === 'video' && element.video_source === 'youtube';
    },
    isYoutubeEmbedSource: function (element) {
      return element.type === 'video' && element.video_source === 'youtube_embed';
    },
    isTwitchVideoSource: function (element) {
      return element.type === 'video' && element.video_source === 'twitch_video';
    },
    isTwitchClipSource: function (element) {
      return element.type === 'video' && element.video_source === 'twitch_clip';
    },
    isBilibiliVideoSource: function (element) {
      return element.type === 'video' && element.video_source === 'bilibili_video';
    },
    batchUpload: function () {
      if (this.loading['BATCH_UPLOADING']) {
        return;
      }
      this.uploadLoadingStatus('BATCH_UPLOADING', true);
      const data = {
        post_serial: this.post.serial,
        url: this.batchString
      };
      let tempRollbackData = this.batchString;
      this.batchString = "";
      axios.post(this.batchCreateEndpoint, data)
        .then(res => {
          let waittime = 0;
          _.forEach(res.data.data, (data) => {
            // this.elements.meta.total++;
            // if (this.totalRow < this.perPage) {
            //   this.elements.data.push(data);
            // }
            setTimeout(() => {
              this.dismissAlert();
              this.showAlert((data.title) ? data.title : this.$t('Uploaded!'), 'success');
            }, waittime);
            waittime += 500;
          });
          this.loadElements(this.currentPage);
        })
        .catch((err) => {
          // console.log(err.response.data);
          // handle 504
          if(err.response.status === 504) {
            this.showAlert(this.$t('edit_post.batch_upload_timeout'), 'danger');
            this.uploadLoadingStatus('BATCH_UPLOADING', false);
            return ;
          }

          let errorUrl = '';
          if (err.response?.data?.data?.error_url) {
            errorUrl = err.response.data.data.error_url + " ";
          }
          if (err.response?.data?.data?.elements) {
            _.forEach(err.response.data.data.elements, (data) => {
              tempRollbackData = tempRollbackData.replace(data.source_url + ',', '')
              tempRollbackData = tempRollbackData.replace(data.source_url, '')
              this.elements.push(data);
            });
          }
          this.showAlert(errorUrl + err.response.data.message, 'danger');
          this.batchString = tempRollbackData.trim();
        })
        .finally(() => {
          this.uploadLoadingStatus('BATCH_UPLOADING', false);
        });
    },
    updateVideoScope: function (index, element, event) {
      let key = event.target.name;
      let seconds = this.toSeconds(event.target.value);
      if (!Number.isInteger(seconds)) {
        return;
      }
      const data = {
        [key]: seconds,
        post_serial: this.post.serial
      };
      const url = this.updateElementEndpoint.replace('_id', element.id);
      axios.put(url, data)
        .then((res) => {
          element[key] = seconds;
          this.$set(this.elements.data, index, element);
          Swal.fire({
            position: "top-end",
            showConfirmButton: false,
            title: this.$t("Updated!"),
            toast: true,
            text: this.$t("The element has been updated."),
            icon: "success",
            timer: 3000
          });
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
    setProgressBarValueFailed: function (file) {
      let filename = file.name;
      this.$set(this.uploadingFiles, filename, -1);
    },

    /** Time **/
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
    getYoutubePlayer(element) {
      return _.get(this.$refs, element.id + '.0.player', null);
    },
    clickYoutubePlayButton(index, element) {
      this.playingVideo = element.id;
      element.loadedVideo = true;
      this.$set(this.elements.data, index, element);

      this.doPlay(element);
    },
    doPlay(element) {
      let player = this.getYoutubePlayer(element);
      if (player) {
        window.player = player;
        player.loadVideoById({
          videoId: element.video_id,
          startSeconds: element.video_start_second,
          endSeconds: element.video_end_second
        });
      }
    },
    clickTwitchPlayButton(index, element) {
      // console.log('clickTwitchPlayButton');
      if(element.video_source === 'twitch_video'){
        let player = this.loadTwitchVideo(index, element);
        
        let isPaused = player.isPaused();
        if(isPaused){
          player.play();
        }
        player.seek(element.video_start_second);
      } else if(element.video_source === 'twitch_clip'){
        element.loadedVideo = true;
        this.$set(this.elements.data, index, element);
      }
    },

    /** Rank **/
    getElementRank(element) {
      if(element.rank !== null) {
        return element.rank.rank;
      }
      return '-';
    },

    playGame() {
      if(this.elements.data.length < 2){
        return;
      }
      window.open(this.playGameRoute, '_blank');
    },
    playButtonOnHoverIn() {
      // console.log('playButtonOnHover');
      if(this.elements.data.length < 2){
        this.showPlayPopover = true;
      }
    },
    playButtonOnHoverOut() {
      // console.log('playButtonOnHoverOut');
      this.showPlayPopover = false;
    },
    handleElementPageChange: function (page) {
      this.loadElements(page);
    },
    share() {
      let url = this.playGameRoute;
      navigator.clipboard.writeText(url).then(() => {
        this.showSharePopover = true;
        if(this.sharePopoverTimeout) {
          clearTimeout(this.sharePopoverTimeout);
        }
        this.sharePopoverTimeout = setTimeout(() => {
          this.showSharePopover = false;
        }, 2000);
      });
    },
    getThumbnailUrl(element) {
      return element.lowthumb_url ? element.lowthumb_url : element.thumb_url;
    },
    loadRankIframe() {
      const iframe = document.getElementById('rank-iframe');
      if(iframe.src === '') {
        iframe.src = this.gameRankRoute; // The URL you want to load
      }
    }
  }
}

</script>

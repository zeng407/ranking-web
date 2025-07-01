<template>
  <div>
    <a href="#" data-toggle="modal" data-target="#uploadGuide">{{ $t('Upload guide') }}&nbsp;<i class="fa-solid fa-circle-info"></i></a>
    <!-- Modal for upload guide -->
    <!-- Modal -->
    <div class="modal fade" id="uploadGuide" data-keyboard="true" tabindex="-1" aria-labelledby="uploadGuidePanel"
      aria-hidden="true">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <ul class="nav nav-pills" id="pills-tab" role="tablist">
              <!-- youtube -->
              <li class="nav-item" role="presentation">
                <button class="nav-link active" id="pills-youtube-tab" data-toggle="pill" data-target="#pills-youtube"
                  type="button" role="tab" aria-controls="pills-youtube" aria-selected="true" style="width: 70px;">
                  <img style="height: 30px; width: 100%; object-fit: contain" alt="yotube logo" src="/storage/youtube-icon.png">
                </button>
              </li>
              <!-- twitch -->
              <li class="nav-item" role="presentation">
                <button class="nav-link" id="pills-twitch-tab" data-toggle="pill" data-target="#pills-twitch"
                  type="button" role="tab" aria-controls="pills-twitch" aria-selected="false" style="width: 70px;">
                  <img style="height: 30px; width: 100%; object-fit: contain;" alt="twitch logo" src="/storage/twitch-icon.svg">
                </button>
              </li>
              <!-- bilibili -->
              <li class="nav-item" role="presentation">
                <button class="nav-link" id="pills-bilibili-tab" data-toggle="pill" data-target="#pills-bilibili"
                  type="button" role="tab" aria-controls="pills-bilibili" aria-selected="false" style="width: 70px;">
                  <img style="height: 30px; width: 100%; object-fit: contain;" alt="bilibili logo" src="/storage/bilibili.ico">
                </button>
              </li>
              <!-- imgur -->
              <li class="nav-item" role="presentation">
                <button class="nav-link" id="pills-imgur-tab" data-toggle="pill" data-target="#pills-imgur"
                  type="button" role="tab" aria-controls="pills-imgur" aria-selected="false" style="width: 70px;">
                  <img style="height: 30px; width: 100%; object-fit: contain;" alt="imgur logo" src="/storage/imgur.svg">
                </button>
              </li>
              <!-- <li class="nav-item" role="presentation">
                <button class="nav-link" id="pills-giphy-tab" data-toggle="pill" data-target="#pills-giphy"
                  type="button" role="tab" aria-controls="pills-giphy" aria-selected="false" style="width: 70px;">
                  <img style="height: 30px; width: 100%; object-fit: contain" alt="giphy logo" src="/storage/giphy-logo-vector.svg"></button>
              </li> -->
              <!-- images -->
              <li class="nav-item" role="presentation">
                <button class="nav-link" id="pills-images-tab" data-toggle="pill" data-target="#pills-images"
                  type="button" role="tab" aria-controls="pills-images" aria-selected="false" style="width: 70px;">
                  <!-- <img style="height: 30px; width: 100%; object-fit: contain" alt="images" src="/storage/image-icon.png"> -->
                  <i class="fa-solid fa-2x fa-photo-film"></i>
                </button>
              </li>
            </ul>
          </div>
          <div class="modal-body">
            <div class="tab-content" id="pills-tabContent">
              <div class="tab-pane fade show active" id="pills-youtube" role="tabpanel" aria-labelledby="pills-youtube-tab">
                <h3>
                  <a href="https://www.youtube.com/" target="_blank">
                    Youtube <i class="fa-solid fa-arrow-up-right-from-square"></i>
                  </a>
                </h3>
                <h5>{{$t('Support formats')}}:</h5>

                <ul>
                  <li>{{$t('Video')}}：https://www.youtube.com/watch?v=xxxxx</li>
                  <li>{{$t('Video')}}：https://www.youtube.com/watch/xxxxx</li>
                  <li>{{$t('Shorts')}}：https://www.youtube.com/shorts/xxxxxx</li>
                  <li>{{$t('Video')}}：https://www.youtube.com/xxxxxx</li>
                  <li>{{$t('Video')}}：https://youtu.be/xxxxx</li>
                </ul>
                <img src="/storage/youtube-link-tutorial.jpg" alt="youtube import" style="width: 100%; object-fit: contain;">
                <h5 class="my-4">{{ $t('Import clips') }}</h5>
                <img src="/storage/youtube-embed-tutorial.jpg" alt="youtube import" style="width: 100%; object-fit: contain;">
                <div class="alert alert-dark">
                  {{ youtubeEmbedSample }}
                </div>
              </div>
              <div class="tab-pane fade" id="pills-bilibili" role="tabpanel" aria-labelledby="pills-bilibili-tab">
                <h3>
                  <a href="https://www.bilibili.com/" target="_blank">
                    bilibili <i class="fa-solid fa-arrow-up-right-from-square"></i>
                  </a>
                </h3>
                <h5>{{$t('Support formats')}}:</h5>
                <ul>
                  <li>{{$t('Video')}}：https://www.bilibili.com/video/BVxxxxxxxxxx/</li>
                </ul>
                <h5>{{ $t('Import bangumi') }}</h5>
                <img src="/storage/bilibili-copy-link.jpg" alt="youtube import" style="width: 100%; object-fit: contain;">
                <div class="alert alert-dark">
                  https://www.bilibili.com/video/BVxxxxxxxxxx/
                </div>
              </div>
              <div class="tab-pane fade" id="pills-twitch" role="tabpanel" aria-labelledby="pills-twitch-tab">
                <h3>
                  <a href="https://www.twitch.tv/" target="_blank">
                    Twitch <i class="fa-solid fa-arrow-up-right-from-square"></i>
                  </a>
                </h3>
                <h5>{{$t('Support formats')}}:</h5>
                <ul>
                  <li>{{$t('VODs')}}：https://www.twitch.tv/videos/xxxxxxx</li>
                  <li>{{$t('Clips')}}：https://www.twitch.tv/xxxxxx/clip/xxxxxx</li>
                </ul>
              </div>
              <div class="tab-pane fade" id="pills-imgur" role="tabpanel" aria-labelledby="pills-imgur-tab">
                <h3>
                  <a href="https://imgur.com/" target="_blank">
                    Imgur <i class="fa-solid fa-arrow-up-right-from-square"></i>
                  </a>
                </h3>
                <h5>{{$t('Support formats')}}:</h5>
                <ul>
                  <li>https://imgur.com/gallery/xxxxx</li>
                  <li>https://imgur.com/a/xxxxx</li>
                  <li>https://imgur.com/xxxxx</li>
                  <li>https://i.imgur.com/xxxxx.jpg</li>
                  <li>https://i.imgur.com/xxxxx.png</li>
                  <li>https://i.imgur.com/xxxxx.gif</li>
                  <li>https://i.imgur.com/xxxxx.mp4</li>
                </ul>
              </div>
              <!-- <div class="tab-pane fade" id="pills-giphy" role="tabpanel" aria-labelledby="pills-giphy-tab">giphy</div> -->
              <div class="tab-pane fade" id="pills-images" role="tabpanel" aria-labelledby="pills-images-tab">
                <h3>{{$t('Images and Videos')}}</h3>
                <h5>{{$t('Support formats')}}:</h5>
                <ul>
                  <li>https://exmaple.com/xxxx.gif</li>
                  <li>https://exmaple.com/xxxx.jpg</li>
                  <li>https://exmaple.com/xxxx.png</li>
                  <li>https://exmaple.com/xxxx.mp4</li>
                </ul>
              </div>

            </div>
          </div>


          <div class="modal-footer mb-sm-0 mb-4">
            <button type="button" class="btn btn-secondary" data-dismiss="modal">{{ $t('Close') }}</button>
          </div>

        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  data() {
    return {
      youtubeEmbedSample: '<iframe src="https://www.youtube.com/embed/xxxxx"  ... ></iframe>'
    };
  },
  methods: {

  },
};
</script>

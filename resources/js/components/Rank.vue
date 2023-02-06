<template>
  <!--  Main -->
  <div class="container">

    <div class="fa-3x text-center" v-if="isLoading">
      <i class="fas fa-spinner fa-spin"></i>
    </div>
    <b-tabs content-class="mt-3" v-if="!isLoading">
      <b-tab title="我的排名" v-if="gameResult">
        <table class="table table-hover" style="table-layout: fixed">
          <thead>
          <tr>
            <th scope="col">#</th>
            <th scope="col" class="w-75"></th>
            <th scope="col">全體排名</th>
          </tr>
          </thead>
          <tbody>
          <tr>
            <th scope="row">1</th>
            <td class="text-center">
              <youtube v-if="isYoutubeSource(gameResult.winner)" width="100%" height="270"
                       :ref="gameResult.winner.id"
                       @ready="doPlay(gameResult.winner)"
                       :player-vars="{
                            controls:1,
                            autoplay:0,
                            start: gameResult.winner.video_start_second,
                            end:gameResult.winner.video_end_second,
                            rel: 0,
                            host: 'https://www.youtube.com'
                            }"
              ></youtube>
              <video v-else-if="isVideoSource(gameResult.winner)" width="100%" height="270" loop autoplay muted
                     playsinline :src="gameResult.winner.source_url"></video>
              <img v-else-if="isImageSource(gameResult.winner)" :src="gameResult.winner.thumb_url" height="300px"
                   alt="gameResult.winner.title">
              <p>{{ gameResult.winner.title }}</p>
            </td>
            <td>{{ gameResult.winner_rank }}</td>
          </tr>
          <tr v-for="(rank, index) in gameResult.data">
            <th scope="row">{{ index + 2 }}</th>
            <td class="text-center">
              <youtube v-if="isYoutubeSource(rank.loser)" width="100%" height="270"
                       :ref="rank.loser.id"
                       @ready="doPlay(rank.loser)"
                       :player-vars="{
                            controls: 1,
                            autoplay: 0,
                            start: rank.loser.video_start_second,
                            end: rank.loser.video_end_second,
                            rel: 0,
                            host: 'https://www.youtube.com'
                            }"
              ></youtube>
              <video v-else-if="isVideoSource(rank.loser)" width="100%" height="270" loop autoplay muted playsinline
                     :src="rank.loser.source_url"></video>
              <img v-else-if="isImageSource(rank.loser)" :src="rank.loser.thumb_url" height="300px"
                   alt="rank.element.title">
              <p>{{ rank.loser.title }}</p>
            </td>
            <td>{{ rank.rank }}</td>
          </tr>
          </tbody>
        </table>
      </b-tab>
      <b-tab title="全體排名">
        <table class="table table-hover d-sm-none d-md-block" style="table-layout: fixed">
          <thead>
          <tr>
            <th scope="col">#</th>
            <th scope="col" class="w-50"></th>
            <th scope="col" style="min-width: 10px">決賽%</th>
            <th scope="col" style="min-width: 10px">勝率%</th>
          </tr>
          </thead>
          <tbody v-if="loadingPage">
          <tr>
            <td colspan="4">
              <div class="fa-3x text-center">
                <i class="fas fa-spinner fa-spin"></i>
              </div>
            </td>
          </tr>
          </tbody>
          <tbody v-if="rankReportData && !loadingPage">
          <tr v-for="(rank, index) in rankReportData.data">
            <th scope="row">{{ rank.rank }}</th>
            <td style="overflow: scroll">
              <div>
                <img :src="rank.element.thumb_url" height="300" alt="rank.element.title"
                     @click="handleClickElement(rank.element)">
                <youtube v-if="isYoutubeSource(rank.element) && rank.element.isPlaying" width="100%" height="270"
                         :ref="rank.element.id"
                         @ready="doPlay(rank.element)"
                         :player-vars="{
                              controls:1,
                              autoplay:0,
                              start: rank.element.video_start_second,
                              end:rank.element.video_end_second,
                              rel: 0,
                              host: 'https://www.youtube.com'
                              }"
                ></youtube>
                <video v-else-if="isVideoSource(rank.element)" width="100%" height="300" loop autoplay muted playsinline
                       :src="rank.element.source_url"></video>
                {{ rank.element.title }}
              </div>
            </td>
            <td>{{ rank.final_win_rate | percent }}</td>
            <td>{{ rank.win_rate | percent }}</td>
          </tr>
          </tbody>
        </table>

        <div class="row">
          <div class="col-12" v-if="rankReportData">
            <b-pagination
              v-model="currentPage"
              :total-rows="rankReportData.meta.total"
              :per-page="rankReportData.meta.per_page"
              first-number
              last-number
              @change="handlePageChange"
              align="center"
            ></b-pagination>
          </div>
        </div>
      </b-tab>
    </b-tabs>


  </div>
</template>

<script>
export default {
  mounted() {
    this.loadGameResult();
    this.loadRankReport();
    this.loadRankData();
  },
  props: {
    postSerial: String,
    getRankEndpoint: String,
    getRankReportEndpoint: String,
    getGameResultEndpoint: String

  },
  computed: {
    isLoading: function () {
      return this.loadingGameResult || this.loadingGameReport;
    }
  },
  data: function () {
    return {
      rankInfo: null,
      rankReportData: {
        data: {},
        meta: {}
      },
      currentPage: 1,
      gameResult: null,
      loadingGameResult: true,
      loadingGameReport: true,
      loadingPage: true
    }
  },
  methods: {
    loadRankData: function () {
      axios.get(this.getRankEndpoint)
        .then(res => {
          this.rankInfo = res.data;
        });
    },
    loadGameResult: function () {
      const urlParams = new URLSearchParams(window.location.search);
      if (!urlParams.has('g')) {
        this.loadingGameResult = false;
        return;
      }
      const api = this.getGameResultEndpoint.replace('_serial', urlParams.get('g'));
      axios.get(api)
        .then(res => {
          this.gameResult = res.data;
        })
        .finally(() => {
          this.loadingGameResult = false;
        });
    },
    loadRankReport: function (page = 1) {
      this.loadingPage = true;
      const filter = {
        'page': page
      };
      axios.get(this.getRankReportEndpoint, {params: filter})
        .then(res => {
          this.rankReportData = res.data;
          this.currentPage = res.data.meta.current_page;
        })
        .finally(() => {
          this.loadingGameReport = false;
          this.loadingPage = false;
        });
    },
    handlePageChange: function (page) {
      this.loadRankReport(page);
    },
    isYoutubeSource: function (element) {
      return element.type === 'video' && element.video_source === 'youtube';
    },
    isVideoSource: function (element) {
      return element.type === 'video';
    },
    isImageSource: function (element) {
      return element.type === 'image';
    },
    handleClickElement: function (element) {
      element.isPlaying = true;
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

  }
}

</script>

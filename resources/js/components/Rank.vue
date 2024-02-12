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
            <th scope="col">其他人的排名</th>
          </tr>
          </thead>
          <tbody>
          <tr>
            <th scope="row">1</th>
            <td>
              <div>
                <img :src="gameResult.winner.thumb_url" height="300px" alt="gameResult.winner.title">

                <a v-if="gameResult.winner.type === 'video'" :href="gameResult.winner.thumb_url" target="_blank">
                  <p>{{ gameResult.winner.title }}</p>
                </a>
                <p v-if="gameResult.winner.type === 'image'">{{ gameResult.winner.title }}</p>
              </div>
            </td>
            <td>{{gameResult.winner_rank}}</td>
          </tr>
          <tr v-for="(rank, index) in gameResult.data">
            <th scope="row">{{ index+2 }}</th>
            <td>
              <div>
                <img :src="rank.loser.thumb_url" height="300px" alt="rank.element.title">

                <a v-if="rank.loser.type === 'video'" :href="rank.loser.thumb_url" target="_blank">
                  <p>{{ rank.loser.title }}</p>
                </a>
                <p v-if="rank.loser.type === 'image'">{{ rank.loser.title }}</p>
              </div>
            </td>
            <td>{{rank.rank}}</td>
          </tr>
          </tbody>
        </table>
      </b-tab>
      <b-tab :title="gameResult ? this.$t('Gloabl Rank'): this.$t('Rank')">
        <table class="table table-hover" style="table-layout: fixed">
          <thead>
          <tr>
            <th scope="col">#</th>
            <th scope="col" class="w-75"></th>
            <th scope="col">{{ $t('Champion') }}%</th>
            <th scope="col">{{ $t('1v1')}}%</th>
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
                <img :src="rank.element.thumb_url" height="300px" alt="rank.element.title">

                <a v-if="rank.element.type === 'video'" :href="rank.element.thumb_url" target="_blank">
                  <p>{{ rank.element.title }}</p>
                </a>
                <p v-if="rank.element.type === 'image'">{{ rank.element.title }}</p>
              </div>
            </td>
            <td>{{rank.final_win_rate | percent}}</td>
            <td>{{rank.win_rate | percent}}</td>
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
      }

    }
  }

</script>

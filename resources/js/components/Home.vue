<template>
  <div class="container-fluid">
    <div class="row justify-content-center pt-sm-4">
      <div class="col-xl-4 pt-2" v-for="post in posts.data">
        <div class="card">
          <div class="row no-gutters">
            <div class="col-6 pr-1">
              <!--              <img class="bd-placeholder-img card-img-top" :src="post.image1.url" :alt="post.image1.title">-->
              <!--              <p class="text-center">{{post.image1.title}}</p>-->
              <div :style="{
                'background': 'url('+post.image1.url+')',
                'width': '100%',
                'height': '300px',
                'background-repeat': 'no-repeat',
                'background-size': 'cover',
                'background-position': 'center center',
                'display': 'flex'}"></div>
              <!--                <img class="bd-placeholder-img card-img-top" :src="post.image1.url" :alt="post.image1.title">-->
              <h3 class="text-center">{{post.image1.title}}</h3>
            </div>
            <div class="col-6 pl-1">
              <div :style="{
                    'background': 'url('+post.image2.url+')',
                    'width': '100%',
                    'height': '300px',
                    'background-repeat': 'no-repeat',
                    'background-size': 'cover',
                    'background-position': 'center center',
                    'display': 'flex'}"></div>

              <h3 class="text-center">{{post.image2.title}}</h3>

            </div>
            <div class="card-body pt-0 text-center">
              <h3 class="card-title">
                <a :href="getGameUrl(post.serial)" target="_blank">{{post.title}}</a>
              </h3>
              <p>{{post.description}}</p>
              <span class="card-text float-right">
                <small
                  class="text-muted">{{moment(post.created_at).format('lll')}}
                </small>
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
  export default {
    mounted() {
      console.log('Component mounted.');
      this.loadData();
    },
    props: [
      'indexPostsEndpoint',
      'playGameRoute'
    ],
    data: function () {
      return {
        posts: []
      }
    },
    methods: {
      loadData: function () {
        console.log(this.indexPostsEndpoint);
        axios.get(this.indexPostsEndpoint)
          .then(res => {
            this.posts = res.data;
          })
      },
      showPost: function (serial) {
        const url = this.playGameRoute.replace('_serial', serial);
        window.open(url, '_blank');
      },
      getGameUrl: function(serial) {
        return this.playGameRoute.replace('_serial', serial);
      }
    }
  }

</script>

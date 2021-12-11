<template>
  <div class="container-fluid">
    <div class="row justify-content-center pt-sm-4">
      <div class="col-xl-4 pt-2" v-for="post in posts.data">
        <div class="card">
          <div class="row no-gutters">
            <div class="col-6 pr-1">
              <img class="bd-placeholder-img card-img-top" :src="post.image1.url" :alt="post.image2.title">
              <p class="text-center">{{post.image1.title}}</p>
              <!--              <div class="col-md-6 pl-1 pr-1">-->
              <!--                <div :style="{-->
              <!--                'background': 'url('+post.image2.url+')',-->
              <!--                'width': '100%',-->
              <!--                'height': '200px',-->
              <!--                'background-repeat': 'no-repeat',-->
              <!--                'background-size': 'cover',-->
              <!--                'background-position': 'center center',-->
              <!--                'display': 'flex'}"></div>-->
              <!--                <img class="bd-placeholder-img card-img-top" :src="post.image2.url" :alt="post.image2.title">-->
              <!--                <p class="text-center">{{post.image2.title}}</p>-->
              <!--              </div>-->
            </div>
            <div class="col-6 pl-1">
              <img class="bd-placeholder-img card-img-top" :src="post.image2.url" :alt="post.image2.title">
              <p class="text-center">{{post.image2.title}}</p>
            </div>
            <div class="card-body pt-0">
              <h3 class="card-title">
                <a href="#" @click="showPost(post.serial)">{{post.title}}</a>
              </h3>
              <p>{{post.description}}</p>
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
        posts: [

        ]
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
        const url = this.playGameRoute.replace('_serial',serial);
        window.open(url, '_blank');
      }
    }
  }

</script>

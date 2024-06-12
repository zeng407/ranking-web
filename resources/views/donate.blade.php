@extends('layouts.app', [
  'ogImage' => asset('/storage/og-image.jpeg'),
  'stickyNav' => 'sticky-top',
  'title' => __('Donate')
])

@section('content')

<div class="container my-5">
    @include('partial.lang', ['langPostfixURL' => url_path_without_locale()])
    <h2 class="text-center mb-4">{{__('Donate')}}</h2>
    <div class="d-flex justify-content-center mb-5" style="height: 300px">
      <img src="https://i.imgur.com/jgJLuWp.png" height="300px">
    </div>
    <p class="text-center mb-5">
      {{__('donate.description')}}
    </p>
    <div class="row justify-content-center">
      <div class="col-12 text-center my-2">
        <h5>{{__('Payment')}}</h5>
        {{-- <img src="https://payment.opay.tw/Content/themes/WebStyle201404/images/allpay.png"/> --}}
        <p>(歐付寶支付系統)</p>
      </div>
      <div class="col-12 col-md-3 text-center mt-2">
        <a target="_blank" href="https://p.opay.tw/bmlRV">
          <img src="https://i.imgur.com/v041xUS.png" height="100px">
          <h5 class="pt-1 text-nowrap">{{__('donate.coffee')}} ($50)</h5>
        </a>
      </div>
      <div class="col-12 col-md-3 text-center mt-2">
        <a target="_blank" href="https://p.opay.tw/l5Idq">
          <img src="https://i.imgur.com/n0tEcz8.jpeg" height="100px">
          <h5 class="pt-1 text-nowrap">{{__('donate.catfood')}} ($100)</h5>
        </a>
        <a href="#" data-toggle="modal" data-target="#catModal" @click.prevent>
          {{__('donate.see_the_cat')}}<i class="fas fa-search"></i>
        </a>
      </div>
      <div class="col-12 col-md-3 text-center mt-2">
        <a target="_blank" href="https://p.opay.tw/r8DGF">
          <img src="https://i.imgur.com/8dZff5T.jpeg" height="100px">
          <h5 class="pt-1 text-nowrap">{{__('donate.lunch')}} ($120)</h5>
        </a>
      </div>
      <div class="col-12 col-md-3 text-center mt-2">
        <a target="_blank" href="https://p.opay.tw/916Gs">
          <img src="https://i.imgur.com/pCq7eP4.jpeg" height="100px">
          <h5 class="pt-1 text-nowrap">{{__('donate.server')}} ($200)</h5>
        </a>
      </div>
      <div class="col-12 col-md-3 text-center mt-2">
        <a target="_blank" href="https://p.opay.tw/79Czd">
          <img src="https://i.imgur.com/2PXmGuk.png" height="100px">
          <h5 class="pt-1 text-nowrap">{{__('donate.whatever')}}</h5>
        </a>
      </div>
    </div>
</div>

<!-- Cat Modal -->
<div class="modal fade" id="catModal" tabindex="-1" aria-labelledby="catModalLabel" aria-hidden="true">
  <div class="modal-dialog">
    <div class="modal-content">
      <div class="modal-header">
        <button type="button" class="close" data-dismiss="modal" aria-label="Close">
          <span aria-hidden="true">&times;</span>
        </button>
      </div>
      <div class="modal-body align-self-center">
        <img src="https://i.imgur.com/unVRUTL.jpeg" class="img-fluid" alt="Cute Cat">
      </div>
    </div>
  </div>
</div>
@endsection


@section('footer')
  @include('partial.footer')
@endsection

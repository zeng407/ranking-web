@if ($message = Session::get('success'))
<span class="alert alert-success alert-dismissible alert-message">
    <button type="button" class="close" data-dismiss="alert">×</button>    
    <strong class="m-2">{{ $message }}</strong>
</span>
@endif
  
@if ($message = Session::get('error'))
<span class="alert alert-danger alert-block fa-pull-right mr-5">
    <button type="button" class="close" data-dismiss="alert">×</button>    
    <strong class="m-2">{{ $message }}</strong>
</span>
@endif
   
@if ($message = Session::get('warning'))
<span class="alert alert-warning alert-block fa-pull-right mr-5">
    <button type="button" class="close" data-dismiss="alert">×</button>    
    <strong class="m-2">{{ $message }}</strong>
</span>
@endif
   
@if ($message = Session::get('info'))
<span class="alert alert-info alert-block fa-pull-right mr-5">
    <button type="button" class="close" data-dismiss="alert">×</button>    
    <strong class="m-2">{{ $message }}</strong>
</span>
@endif
  
@if ($errors->any())
<span class="alert alert-danger alert-block fa-pull-right mr-5">
    <button type="button" class="close" data-dismiss="alert">×</button>    
    Something went wrong, please check and try again!
    @env('local')
    <ul>
        @foreach ($errors->all() as $error)
            <li>{{ $error }}</li>
        @endforeach
    </ul>
    @endenv
    
</span>
@endif
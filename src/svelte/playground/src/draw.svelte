<script>
export let DrawApp;
DrawApp = {
    Utility:{
        callBranch:function(branchName){
            var promise = new  DrawApp.sdk.makeDeferred()
            DrawApp.sdk.call.apply(DrawApp.sdk,arguments).done(function(result){
                promise.resolve(result)
            }).fail(function(err){
                console.warn('Failure to call',branchName)
                console.warn(err)
            })
            return promise
        }
    }
    ,Event:{
        onLoginSucceed:function(sdk){
            DrawApp.sdk = sdk
            sdk.useTree('SBS','/ws').done(function(){
                /*
                DrawApp.callBranch('Draw.Ping').done(function(result){
                    console.log('Ping',result)
                })
                */
                DrawApp.Event.onTreeConnected()
            }).fail(function(err){
                console.log('user tree not ok',err)
            })
        }
        ,onTreeConnected:function(){
            // Create DrawUser and initialize it by UserData on server
            DrawApp.drawUser = new DrawUser(DrawApp.sdk)
            // Load data from servers
            DrawApp.callBranch('Draw.UserState').done(function(userdata){
                console.log('UserData:',userdata)
                DrawApp.drawUser.setUserState(userdata)
            })
            // enable DOMElement-data bindings
            //new Vue({
            //    el:'.username',
            //    data:DrawApp.drawUser.user
            //})
        }    
    }
}
//shortcuts
DrawApp.callBranch = DrawApp.Utility.callBranch
</script>

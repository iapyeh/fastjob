
# for Javascript (don't work)
#protoc --js_out=library=myprotos_lib.js,binary:.  sidebyside.proto
#browserify myprotos_lib.js  -o pb.js

# for Javascript
protoc --js_out=import_style=commonjs,binary:.  objshpb.proto 
browserify objshpb_pb.js  -o objshpb.js
mv objshpb.js ../static/file
rm objshpb_pb.js

# for Golang
protoc --go_out=. objshpb.proto
mv objshpb.pb.go ../model/

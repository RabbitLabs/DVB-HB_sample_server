set VIDEO_IN=bunny.mov
set VIDEO_OUT=master
set HLS_TIME=4
set FPS=25
set GOP_SIZE=100
set PRESET_P=veryfast
set CRF_P=21
set PRESET_P=veryslow
set V_SIZE_1=768x432
set V_SIZE_2=1280x720
set V_SIZE_3=1920x1080

ffmpeg -i %VIDEO_IN% ^
    -preset %PRESET_P% -keyint_min %GOP_SIZ%E -g %GOP_SIZE% -sc_threshold 0 ^
    -r %FPS% -c:v libx264 -pix_fmt yuv420p -c:a aac -b:a 128k -ac 1 -ar 44100 ^
    -map v:0 -s:1 %V_SIZE_1% -b:v:1 1.1M -maxrate:4 1.17M -bufsize:4 2M ^
    -map v:0 -s:2 %V_SIZE_2% -b:v:2 4.5M -maxrate:6 4.8M -bufsize:6 8M ^
    -map v:0 -s:3 %V_SIZE_3% -b:v:3 7.8M -maxrate:8 8.3M -bufsize:8 14M ^
    -map 0:a ^
    -init_seg_name init$RepresentationID$.$ext$ -media_seg_name chunk$RepresentationID$-$Number%%05d$.$ext$ ^
    -use_template 1 -use_timeline 1  ^
    -seg_duration 4 -adaptation_sets "id=0,streams=v id=1,streams=a" ^
    -f dash Dash/dash.mpd
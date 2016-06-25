data=$(/sbin/ifconfig eth0 | grep bytes | awk '{gsub("bytes:","",$2);gsub("bytes:","",$6);a=int($2/1024);b=int($6/1024);print a":"b}')
date=$(date +%s)
now=$(date | awk '{gsub(":","\\:",$0);print $0}')
final="$date:$data"
df=$(df -h | grep "/" -m1 | awk '{print $3"/"$4}')

cd /var/www/go/assets

rrdtool update eth0.rrd $final
rrdtool graph ./test.png -X 0 --end now --start end-43200s --width 500 --height 250 -t "eth0 - traffic - 12 hrs (2 min avg)" \
        --x-grid MINUTE:20:HOUR:1:HOUR:3:0:%H:%M \
        DEF:ds0=eth0.rrd:RX:AVERAGE \
        DEF:ds1=eth0.rrd:TX:AVERAGE \
        VDEF:ds0max=ds0,MAXIMUM \
        VDEF:ds0avg=ds0,AVERAGE \
        VDEF:ds0min=ds0,MINIMUM \
        VDEF:ds1max=ds1,MAXIMUM \
        VDEF:ds1avg=ds1,AVERAGE \
        VDEF:ds1min=ds1,MINIMUM \
        VDEF:ds1last=ds1,LAST \
        VDEF:ds0last=ds0,LAST \
        AREA:ds1#19A9DA:"Outgoing\:\t\g" COMMENT:"Max\:" GPRINT:ds1max:"%.2lf KB/s\t\g" COMMENT:"Avg\:" GPRINT:ds1avg:"%.2lf KB/s\t\g" COMMENT:"Last\:" GPRINT:ds1last:"%.2lf KB/s\l" \
        CDEF:shading_rx_9=ds1,0.9,* \
        CDEF:shading_rx_85=ds1,0.85,* \
        CDEF:shading_rx_8=ds1,0.8,* \
        CDEF:shading_rx_75=ds1,0.75,* \
        CDEF:shading_rx_7=ds1,0.7,* \
        CDEF:shading_rx_65=ds1,0.65,* \
        CDEF:shading_rx_6=ds1,0.6,* \
        CDEF:shading_rx_55=ds1,0.55,* \
        CDEF:shading_rx_5=ds1,0.5,* \
        CDEF:shading_rx_45=ds1,0.45,* \
        CDEF:shading_rx_4=ds1,0.4,* \
        CDEF:shading_rx_35=ds1,0.35,* \
        CDEF:shading_rx_3=ds1,0.3,* \
        CDEF:shading_rx_2=ds1,0.2,* \
        CDEF:shading_rx_1=ds1,0.1,* \
         AREA:shading_rx_9#159ECB \
        AREA:shading_rx_85#159AC6 \
         AREA:shading_rx_8#1496C2 \
        AREA:shading_rx_75#1493BD \
         AREA:shading_rx_7#138FB9 \
        AREA:shading_rx_65#138CB4 \
         AREA:shading_rx_6#1288AF \
        AREA:shading_rx_55#1285AB \
         AREA:shading_rx_5#1181A6 \
        AREA:shading_rx_45#117DA2 \
         AREA:shading_rx_4#107A9D \
        AREA:shading_rx_35#0F7395 \
         AREA:shading_rx_3#0E6F90 \
         AREA:shading_rx_2#0D6887 \
         AREA:shading_rx_1#0D6887 \
        LINE2:ds0#03C1C1:"Incoming\:\t\g" COMMENT:"Max\:" GPRINT:ds0max:"%.2lf KB/s\t\g" COMMENT:"Avg\:" GPRINT:ds0avg:"%.2lf KB/s\t\g" COMMENT:"Last\:" GPRINT:ds0last:"%.2lf KB/s\l" \
        COMMENT:" \l" \
        COMMENT:"  Disk Usage\:" COMMENT:"$df\l" \
        COMMENT:"$now\r"
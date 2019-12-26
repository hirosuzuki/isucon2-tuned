from __future__ import with_statement

from flask import (
    Flask, request, redirect,
    render_template_string,
    render_template, _app_ctx_stack, Response
)

import json, os
import redis
import datetime

config = {}

app = Flask(__name__, static_url_path='')

from jinja2 import Environment, FileSystemLoader, FileSystemBytecodeCache

app.jinja2env = Environment(
    loader=FileSystemLoader('./templates'),
    bytecode_cache=FileSystemBytecodeCache(directory='./templates_compiled', pattern='%s.cache')
)

HTML_BASE = '/opt/isucon2/cache'

def load_config():
    global config
    print("Loading configuration")
    env = os.environ.get('ISUCON_ENV') or 'local'
    with open('../config/common.' + env + '.json') as fp:
        config = json.load(fp)

ARTIST = {
    1: { "id": 1, "name": "NHN48" },
    2: { "id": 2, "name": "はだいろクローバーZ" }
}

TICKET = {
    1: { "id": 1, "artist": ARTIST[1], "name": "西武ドームライブ" },
    2: { "id": 2, "artist": ARTIST[1], "name": "東京ドームライブ" },
    3: { "id": 3, "artist": ARTIST[2], "name": "さいたまスーパーアリーナライブ" },
    4: { "id": 4, "artist": ARTIST[2], "name": "横浜アリーナライブ" },
    5: { "id": 5, "artist": ARTIST[2], "name": "西武ドームライブ" },
}

VARIATION = {
    1: { "id": 1, "ticket": TICKET[1], "name": "アリーナ席" },
    2: { "id": 2, "ticket": TICKET[1], "name": "スタンド席" },
    3: { "id": 3, "ticket": TICKET[2], "name": "アリーナ席" },
    4: { "id": 4, "ticket": TICKET[2], "name": "スタンド席" },
    5: { "id": 5, "ticket": TICKET[3], "name": "アリーナ席" },
    6: { "id": 6, "ticket": TICKET[3], "name": "スタンド席" },
    7: { "id": 7, "ticket": TICKET[4], "name": "アリーナ席" },
    8: { "id": 8, "ticket": TICKET[4], "name": "スタンド席" },
    9: { "id": 9, "ticket": TICKET[5], "name": "アリーナ席" },
    10: { "id": 10, "ticket": TICKET[5], "name": "スタンド席" },
}

for e in ARTIST.values():
    e["ticket"] = [t for t in TICKET.values() if t["artist"] == e]

for e in TICKET.values():
    e["artist_name"] = e["artist"]["name"]
    e["variation"] = [t for t in VARIATION.values() if t["ticket"] == e]

for e in VARIATION.values():
    e["ticket_name"] = e["ticket"]["name"]
    e["artist"] = e["ticket"]["artist"]
    e["artist_name"] = e["artist"]["name"]

redis_conn = redis.Redis(host='localhost', port=6379, db=0)

def init_db():
    redis_conn.delete("history")
    redis_conn.delete("recent")
    for variation in VARIATION.values():
        redis_conn.set("sold_%0d" % variation["id"], 0)

def get_recent_sold():
    rows = redis_conn.lrange("recent",0, 9)
    recent_sold = []
    for row in rows:
        vs = row.decode("utf-8").split(":")
        recent_sold.append({
            "seat_id": vs[0],
            "v_name": vs[1],
            "t_name": vs[2],
            "a_name": vs[3],
        })
    return recent_sold

def render_to_string(template_name, **context):
    template = app.jinja2env.get_template(template_name)
    result = template.render(**context)
    return result

def buy_page_request_order(member_id, index, variation_id):
    redis_conn.rpush("history", "%s,%s,%s,%s" % (member_id, "%02d-%02d" % (index // 64, index % 64), variation_id, datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")))
    return

def buy_page_inc_stock_count(variation_id, maxnum=4096):
    key = "sold_%0d" % variation_id
    n = redis_conn.incr(key)
    if n > maxnum:
        redis_conn.decr(key)
        return -1
    return n

@app.route("/buy", methods=['POST'])
def buy_page():

    variation_id = int(request.values['variation_id'])
    member_id = request.values['member_id']

    vari = VARIATION[variation_id]

    sold_count = buy_page_inc_stock_count(variation_id)
    if sold_count < 0:
        return render_template('soldout.html')

    index = sold_count - 1
    buy_page_request_order(member_id, index, variation_id)

    seat_id = "%02d-%02d" % (index // 64, index % 64)

    redis_conn.lpush("recent", "%s:%s:%s:%s" % (seat_id, vari['name'], vari['ticket_name'], vari['artist_name']))
    redis_conn.ltrim("recent", 0, 9)

    return render_template('complete.html', seat_id=seat_id, member_id=member_id)

def file_write(filename, text):
    tmpFile = HTML_BASE + '/tmp.html.%d' % os.getpid()
    f = open(tmpFile, 'w')
    f.write(text)
    f.flush()
    f.close()
    os.rename(tmpFile, filename)

def create_side_html():
    html = render_to_string('side.html', recent_sold=get_recent_sold())
    file_write(HTML_BASE + '/side.html', html)

def create_top_html():
    html = render_to_string('index.html', artists=ARTIST.values())
    file_write(HTML_BASE + '/index.html', html)

def create_ticket_html(ticket_id):

    ticket = TICKET[ticket_id]

    variations = []
    for v in ticket["variation"]:
        v["sold_count"] = int(redis_conn.get("sold_%0d" % v["id"]))
        variations.append(v)

    for variation in variations:
        variation['vacancy'] = 4096 - variation['sold_count']
        html = ""
        i = 0
        for row in range(64):
            html += "<tr>\n"
            for col in range(64):
                key = "%02d-%02d" % (row, col)
                state = "unavailable" if i < variation['sold_count'] else "available"
                html += '<td id="%s" class="%s"></td>\n' % (key, state)
                i += 1
            html += "</tr>"
        variation['html'] = html


    html = render_to_string('ticket.html', ticket=ticket, variations=variations)
    file_write(HTML_BASE + '/ticket/%d' % ticket_id, html)

def create_artist_html(artist_id):
    artist = ARTIST[artist_id]
    tickets = [{
        "id": t["id"],
        "name": t["name"],
        "count": sum([4096 - int(redis_conn.get("sold_%0d" % v["id"])) for v in t["variation"]])
    } for t in artist["ticket"]]
    html = render_to_string('artist.html', artist=artist, tickets=tickets)
    file_write(HTML_BASE + '/artist/%d' % artist_id, html)

def init_cache_html():
    create_side_html()
    create_top_html()
    for ticket in TICKET.values():
        create_ticket_html(ticket["id"])
    for artist in ARTIST.values():
        create_artist_html(artist["id"])

@app.route("/update", methods=['GET'])
def html_update():
    init_cache_html()
    return "OK"

@app.route("/admin", methods=['GET', 'POST'])
def admin_page():
    if request.method == 'POST':
        init_db()
        redis_conn.delete('recent')
        init_cache_html()
        return redirect("/admin")
    else:
        redis_conn.delete('recent')
        init_cache_html()
        return render_template('admin.html')

@app.route("/admin/order.csv")
def admin_csv():
    body = ''
    for i, row in enumerate(redis_conn.lrange("history", 0, -1)):
        body += str(i + 1) + "," + row.decode("utf-8") + "\n"
    return Response(body, content_type="text/csv")

if __name__ == "__main__":
    load_config()
    port = int(os.environ.get("PORT", '5000'))
    app.run(debug=1, host='0.0.0.0', port=port)
else:
    load_config()

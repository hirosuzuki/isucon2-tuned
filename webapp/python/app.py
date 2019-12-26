from __future__ import with_statement

import MySQLdb
from MySQLdb.cursors import DictCursor

from flask import (
    Flask, request, redirect,
    render_template_string,
    render_template, _app_ctx_stack, Response
)

import json, os
import redis

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

def connect_db():
    global config
    host = config['database']['host']
    port = config['database']['port']
    username = config['database']['username']
    password = config['database']['password']
    dbname   = config['database']['dbname']
    print("Connect MySQL")
    db = MySQLdb.connect(host=host, port=port, db=dbname, user=username, passwd=password, cursorclass=DictCursor, charset="utf8")
    return db

def init_db():
    print("Initializing database")
    db = get_db() 
    cur = db.cursor()
    with open('../config/database/init_data.sql') as fp:
        for line in fp:
            line = line.strip()
            if line:
                cur.execute(line)
        db.commit()
          

def get_recent_sold():
    redis = get_redis()
    rows = redis.lrange("recent",0, 9)
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

cache = {}

def get_db():
    if not 'db' in cache:
        cache['db'] = connect_db()
    return cache['db']

def get_redis():
    if not 'redis' in cache:
        cache['redis'] = redis.Redis(host='localhost', port=6379, db=0)
    return cache['redis']

def get_variation():
    if not 'variation' in cache:
        variation = {}
        cur = get_db().cursor()
        cur.execute('''select variation.*, ticket.name as ticket_name, artist_id, artist.name as artist_name,
            (select min(id) from stock where stock.variation_id = variation.id) as min_stock_id
            from variation, ticket, artist
            where variation.ticket_id = ticket.id and ticket.artist_id = artist.id
            order by variation.id''')
        for row in cur.fetchall():
            variation[row['id']] = row
        cur.close()
        cache['variation'] = variation
    return cache['variation']

def get_artists():
    if not 'artists' in cache:
        cur = get_db().cursor()
        cur.execute('SELECT * FROM artist')
        artists = cur.fetchall()
        cache['artists'] = artists
        cur.close()
    return cache['artists']

def render_to_string(template_name, **context):
    template = app.jinja2env.get_template(template_name)
    result = template.render(**context)
    return result

def get_stocks(id):
    if not id in cache:
        cur = get_db().cursor()
        cur.execute(
            'SELECT seat_id, null as order_id FROM stock WHERE variation_id = %s order by id',
            (id,)
        )
        cache[id] = cur.fetchall()
        cur.close()
    return cache[id]

def get_ticket(ticket_id):
    if not id in cache:
        cur = get_db().cursor()
        cur.execute(
            'SELECT t.*, a.name AS artist_name FROM ticket t INNER JOIN artist a ON t.artist_id = a.id WHERE t.id = %s LIMIT 1',
            (ticket_id,)
        )
        cache[id] = cur.fetchone()
        cur.close()
    return cache[id]


def buy_page_request_order(db, cur, member_id, index, variation_id):
    cur.execute(
        'INSERT INTO order_request (member_id, seat_id, variation_id) VALUES (%s, %s, %s)',
        (member_id, "%02d-%02d" % (index // 64, index % 64), variation_id)
    )
    return db.insert_id()

def buy_page_inc_stock_count(db, cur, variation_id):
    cur.execute(
        'UPDATE variation SET sold_count = last_insert_id(sold_count + 1) WHERE id = %s',
        (variation_id,)
    )
    return db.insert_id()


@app.route("/buy", methods=['POST'])
def buy_page():

    variation_id = int(request.values['variation_id'])
    member_id = request.values['member_id']

    variation = get_variation()
    vari = variation[variation_id]

    db = get_db()
    cur = db.cursor()

    sold_count = buy_page_inc_stock_count(db, cur, variation_id)
    if sold_count > 4096:
        db.rollback()
        return render_template('soldout.html')

    index = sold_count - 1
    order_id = buy_page_request_order(db, cur, member_id, index, variation_id)

    seat_id = "%02d-%02d" % (index // 64, index % 64)
    db.commit()

    redis = get_redis()
    redis.lpush("recent", "%s:%s:%s:%s" % (seat_id, vari['name'], vari['ticket_name'], vari['artist_name']))
    redis.ltrim("recent", 0, 9)

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
    artists = get_artists()
    html = render_to_string('index.html', artists=artists)
    file_write(HTML_BASE + '/index.html', html)

def create_ticket_html(ticket_id):
    cur = get_db().cursor()
   
    ticket = get_ticket(ticket_id)

    cur.execute(
        'SELECT id, name, sold_count FROM variation WHERE ticket_id = %s',
        (ticket_id,)
    )
    variations = cur.fetchall()

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
    cur = get_db().cursor()

    cur.execute('SELECT id, name FROM artist WHERE id = %s LIMIT 1', (artist_id,))
    artist = cur.fetchone()

    cur.execute('SELECT id, name FROM ticket WHERE artist_id = %s', (artist_id,))
    tickets = cur.fetchall()

    for ticket in tickets:
        cur.execute(
            '''SELECT sum(4096 - sold_count) as cnt FROM variation
                WHERE variation.ticket_id = %s''',
            (ticket['id'],)
        )
        ticket['count'] = cur.fetchone()['cnt']

    cur.close()

    html = render_to_string('artist.html', artist=artist, tickets=tickets)
    file_write(HTML_BASE + '/artist/%d' % artist_id, html)

def init_cache_html():
    variation = get_variation()
    create_side_html()
    create_top_html()
    ticket_ids = set([e['ticket_id']  for e in variation.values()])
    for ticket_id in ticket_ids:
        create_ticket_html(ticket_id)
    artist_ids = set([e['artist_id']  for e in variation.values()])
    for artist_id in artist_ids:
        create_artist_html(artist_id)

@app.route("/update", methods=['GET'])
def html_update():
    init_cache_html()
    return "OK"

@app.route("/admin", methods=['GET', 'POST'])
def admin_page():
    if request.method == 'POST':
        init_db()
        redis = get_redis()
        redis.delete('recent')
        init_cache_html()
        return redirect("/admin")
    else:
        redis = get_redis()
        redis.delete('recent')
        init_cache_html()
        return render_template('admin.html')

@app.route("/admin/order.csv")
def admin_csv():
    cur = get_db().cursor()
    cur.execute('''SELECT order_request.* FROM order_request ORDER BY order_request.id ASC''')
    orders = cur.fetchall()
    cur.close()

    body = ''
    for order in orders:
        body += ','.join([str(order['id']), order['member_id'], order['seat_id'], str(order['variation_id']), order['updated_at'].strftime('%Y-%m-%d %X')])
        body += "\n"
    return Response(body, content_type="text/csv")

if __name__ == "__main__":
    load_config()
    port = int(os.environ.get("PORT", '5000'))
    app.run(debug=1, host='0.0.0.0', port=port)
else:
    load_config()


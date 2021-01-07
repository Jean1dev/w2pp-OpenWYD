//#define EF_NONE     			  0	// * EFFECT Type�� 0 �̸� ����Ʈ�� ����.  ITEM���� Score[0]�� ���� ���Ҽ� ����. Nada
// Score ����
#define EF_LEVEL     			  1	// ���� * Score[1] - Score[15] Bonus Score���� �״�� ���� ������. Level Necess�rio
#define EF_DAMAGE                 2 // Aumento de dano
#define EF_AC       			  3	// Aumento de defesa
#define EF_HP        			  4	// Aumento de HP
#define EF_MP       			  5	// Aumento de MP
#define EF_EXP      			  6	// ����
#define EF_STR      		      7	// Aumento de for�a
#define EF_INT      		      8	// Aumento de intelig�ncia
#define EF_DEX       		      9	// Aumento de destreza
#define EF_CON      		     10	// Aumento de constitui��o
#define EF_SPECIAL1    		     11	// Aumento de especial 1
#define EF_SPECIAL2    			 12	// Aumento de especial 2
#define EF_SPECIAL3    			 13	// Aumento de especial 3
#define EF_SPECIAL4    			 14	// Aumento de especial 4
// Score ��.
#define EF_SCORE14     			 15	// 
#define EF_SCORE15     			 16	// 
////////////////      REQUIREMENT        //////////////////////////////////////////
#define EF_POS                   17  // ��ġ ������ �ִ� ��� bit��         - ����Ʈ NOWHERE (0��Ʈ �� EQUIP 0)
#define EF_CLASS                 18  // ���� ������ �ִ� (����)Ŭ���� bit�� - ����Ʈ YES
#define EF_R1SIDC                19  // �Ѽ� 1�ڵ� �Ǵ� ������ ������ �ִ� STR INT DEC CON         - ����Ʈ 0
#define EF_R2SIDC                20  // �μ� 2�ڵ�          �� ������ �ִ� STR INT DEC CON         - ����Ʈ �Ұ�
////////////////   BONUS  ////////////////////////////////
#define EF_WTYPE         	     21	 // ���� ����з� (�ִϸ��̼�) 
//  1: Caliburn (Todos Battle)
//  2: �߰�(���̽�)    
//  3: �den (TK Battle)           

// 11: Balmungs (Todos Battle)      
// 12: ? (?)            
// 13: Basileus (TK Battle)             

// 21: Lan�as (BM e TK Magos)              
// 22: ��â               
// 23: Foice do lich (?)             

// 31: Cetro (FM Cancel)      
// 32: Cajado (FM Black)       
// 33: ?         

// 41: Garra (HT Battle)

// 51: �ҹ���

// 61: ��Ŭ��
// 62: ��Ŭ��
// 63: ��Ŭ�� 

// 101: Arcos (Todos Battle)
// 102: Dardo m�stico (?)
// 103: Ataque � dist�ncia (BM Battle)
// 104: Lan�a de arremesso (?)
//
// Unique Index
//	45: Caliburn
//	41: Balmung
//	
//	42: Arco
//  43: Garra
//	46: Cruz
//	
//	48: Sabre
//	49: Martelo
//	
//	44: Lan�a
//	47: Cetro
//	47: Cajado
//	
//	51: Escudo
////////////////////////////////////////////////////////////////////////////
#define EF_REQ_STR               22  // ���� 
#define EF_REQ_INT               23  // ����
#define EF_REQ_DEX               24  // ����
#define EF_REQ_CON               25  // ����
/////////////////////////////////////////////////////////////////////////
// * �� �䱸ġ�̳�, �������� �⺻ SIDC�� �߰��Ǵ� ���̴�.
/////////////////////////////////////////////////////////////////////////


#define EF_ATTSPEED  	       	 26	 // ���� ���ݼӵ�
#define EF_RANGE                 27  // ���� �����Ÿ�. ��ü�� �ִ밪�� �����.
////#define EF_PRICE				 28	 // ����
#define EF_RUNSPEED              29  // ���� �޸���ӵ�.
#define EF_SPELL    	    	 30	 // ���� Item�� �԰ų� �Ծ����� �ɸ��� ����
#define EF_DURATION              31  // ���� Item �Ķ��Ÿ1 - ������ ȿ���� �߻��ϴ� �ð�, �Դ¾������� �׻� 
#define EF_PARM2     			 32	 // �Ķ� Item �Ķ��Ÿ2
#define EF_GRID                  33  // ũ�� Carry�� �����ϴ� �����۱׸���
#define EF_GROUND                34  // ��ĭ ������� ���� �����ϴ� ũ��
#define EF_CLAN                  35  // ����
#define EF_HWORDCOIN             36  // �뵷 
#define EF_LWORDCOIN             37  // �ҵ� 
#define EF_VOLATILE              38  // �Ҹ� ������ �ٷ� ������� ������.  0:�Ϲ��屸  1:�������  2:������   3:���� 4:��1  5:��2  6:��3
//                               6:����+15   7:����+1
#define EF_KEYID                 39  // Ű��
#define EF_PARRY                 40  // ȸ�� ȸ���� ������
#define EF_HITRATE               41  // ���� ���߷� ������
#define EF_CRITICAL              42  // ũ�� ũ��Ƽ��
#define EF_SANC                  43  // ���� Sanctuary �ַ� ������ ����Ʈ�� ���Ǹ� 
#define EF_SAVEMANA              44  // ���� 1 -> 99%
#define EF_HPADD                 45  // �ǻ� 1 -> 101%   MAX_MP 
#define EF_MPADD                 46  // ���� 1 -> 101%   MAX_MP
#define EF_REGENHP               47  // ��ȸ
#define EF_REGENMP               48  // ��ȸ
#define EF_RESIST1               49  // ����
#define EF_RESIST2               50  // ����
#define EF_RESIST3               51  // ����
#define EF_RESIST4               52  // ����
#define EF_ACADD                 53  // ����
#define EF_RESISTALL             54  // ����
#define EF_BONUS                 55  // ����
#define EF_HWORDGUILD            56  // Ataque PvP
#define EF_LWORDGUILD            57  // Defesa PvP
#define EF_QUEST                 58  // �ӹ� �ݵ�� 1���� ����, ��κ� Volatile�� �԰� ���õ� �� ��.
#define EF_UNIQUE                59  // ����
#define EF_MAGIC                 60  // ���� 1 -> 1% ���� 
#define EF_AMOUNT                61  // ����
#define EF_HWORDINDEX            62  // ��ȣ
#define EF_LWORDINDEX            63  // ��ȣ
#define EF_INIT1                 64  // �ʱ�
#define EF_INIT2                 65  // �ʱ�
#define EF_INIT3                 66  // �ʱ�
#define EF_DAMAGEADD             67  // ���
#define EF_MAGICADD              68  // ����
// ���� 2���� ���ε� ������ ������ �ʰ� ���̽��� ������ ������ ���� �ɿ��� �ڵ� üŷ�ϴ� �κ�.   
// Ŭ���̾�Ʈ ǥ��ô� �̰��� ���� ������ �̰��� ������ ���̽��� ������ �ʴ´�.
#define EF_HPADD2                69  // ��2   �ǻ��� ������ �⺻ �ɿ� ���ԵǾ� ǥ�õȴ�.
#define EF_MPADD2                70  // ��2   �ǻ��� ������ �⺻ �ɿ� ���ԵǾ� ǥ�õȴ�.
#define EF_CRITICAL2             71  // ũ�� ũ��Ƽ��
#define EF_ACADD2                72  // ����
#define EF_DAMAGE2               73  // 
#define EF_SPECIALALL            74  // �����͸� �������� ���� 

#define	EF_CURKILL				 75  // not used
#define EF_LTOTKILL				 76  // not used
#define EF_HTOTKILL				 77  // not used
#define EF_INCUBATE				 78  // ��ȭ�Ӱ�ġ


#define EF_MOUNTLIFE			 79  // �� �� ���� 
#define EF_MOUNTHP				 80  // �� �� ��
#define EF_MOUNTSANC			 81  // �� ���嵵
#define EF_MOUNTFEED			 82  // �� ��θ� ����	
#define EF_MOUNTKILL			 83  // ���� ���� ���� ����?

#define EF_INCUDELAY             84
#define EF_SUBGUILD				 85	// SUB���, 0�̸� �ִ��� 1,2,3���� �ִ�

#define EF_ITEMLEVEL			 87
#define EF_DONATE				 91

#define EF_GRADE0				 100 // A
#define EF_GRADE1				 101 // B
#define EF_GRADE2				 102 // C
#define EF_GRADE3				 103 // D
#define EF_GRADE4				 104 // E
#define EF_GRADE5				 105 // E Arch

#define EF_WDAY					 106
#define EF_HOUR					 107
#define EF_MIN					 108
#define EF_YEAR					 109
#define EF_WMONTH				 110

#define EF_MOBTYPE				 112
#define EF_ITEMTYPE				 113

#define EF_NOSANC				 126
#define EF_NOTRADE				 127